package site

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fn "github.com/acquia/fn-drupal-operator/pkg/apis/fnresources/v1alpha1"
	environ "github.com/acquia/fn-drupal-operator/pkg/controller/drupalenvironment"
	"github.com/acquia/fn-drupal-operator/pkg/customercontainer"
)

// The common interface for all types of Jobs that can be run on a Site
type JobBuilder interface {
	Label() string                                // the label (annotation) used to identify this job type
	GetJob(*requestHandler, []string) batchv1.Job // build K8s Job based on various inputs
}

// Job run by customers, can run arbitrary and potentially dangerous code.
type CustomerJob struct {
	command []string
}

// Job run as root. Should only be used to run trusted code.  Necessary for some internal workflows.
type RootJob struct {
	command []string
}

func (job *CustomerJob) Label() string { return fn.LabelPrefix + "runJob" }
func (job *RootJob) Label() string     { return fn.LabelPrefix + "runRootJob" }

func (rh *requestHandler) customerJobSpec(command []string) batchv1.JobSpec {
	completions := int32(1)
	// TODO: Needs to be able to be set by user
	activeDeadlineSeconds := int64(3600) // job has one hour to complete or it will be killed
	// ttlSecondsAfterFinished := int32(300)

	customerContainer := customercontainer.Template(rh.app, rh.env)
	customerContainer.Command = command
	customerContainer.Name = "main"

	terminationGracePeriodSeconds := int64(30)

	return batchv1.JobSpec{
		Completions:           &completions,
		ActiveDeadlineSeconds: &activeDeadlineSeconds,
		// TTLSecondsAfterFinished: &ttlSecondsAfterFinished,

		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: rh.site.ChildLabels(),
			},
			Spec: v1.PodSpec{
				RestartPolicy: v1.RestartPolicyOnFailure,
				Containers: []v1.Container{
					customerContainer,
				},
				NodeSelector: map[string]string{
					"function": "workers",
				},
				Volumes: []v1.Volume{
					environ.PhpConfigVolume(),
					environ.DomainMapSecretVolume(),
					customercontainer.FilesVolume(rh.env),
				},
				TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			},
		},
	}
}

func (rh *requestHandler) CustomerCronJob(cron fn.CronSpec) batchv1b1.CronJob {
	failedJobsHistoryLimit := int32(1)
	successfulJobsHistoryLimit := int32(3)
	startingDeadlineSeconds := int64(900)

	// default concurrencyPolicy is Forbid
	concurrencyPolicy := cron.ConcurrencyPolicy
	if concurrencyPolicy == "" {
		concurrencyPolicy = batchv1b1.ForbidConcurrent
	}

	labels := rh.site.ChildLabels()
	labels["type"] = "cron"

	return batchv1b1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cron.Name,
			Namespace: rh.env.Namespace,
			Labels:    labels,
		},
		Spec: batchv1b1.CronJobSpec{
			FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
			Suspend:                    &cron.Suspend,
			StartingDeadlineSeconds:    &startingDeadlineSeconds,

			ConcurrencyPolicy: concurrencyPolicy,

			Schedule: cron.Schedule,

			JobTemplate: batchv1b1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: rh.customerJobSpec(cron.Command),
			},
		},
	}
}

func (rh *requestHandler) onDemandJob(name string, command []string) batchv1.Job {
	ttlSecondsAfterFinished := int32(10)
	spec := rh.customerJobSpec(command)
	spec.TTLSecondsAfterFinished = &ttlSecondsAfterFinished

	labels := rh.site.ChildLabels()
	labels["type"] = "on-demand"

	// generate consistent identifier for this command
	// combine entire command into a list and hash it. This is done to prevent a race condition
	// that can create duplicate jobs when you scale up the cluster to a certain point.
	// By generating the same name for the same command every time, a subsequent Create will fail.
	// There may be a better way: see https://backlog.acquia.com/browse/FN-180
	//
	// In the meantime, this means that the job must be removed before another job with the same command can be run.
	hashcmd := fnv.New32()
	_, _ = hashcmd.Write([]byte(strings.Join(command, " ")))
	nameSuffix := hashcmd.Sum32()

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", name, nameSuffix),
			Namespace: rh.env.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"executable": command[0],
			},
		},
		Spec: spec,
	}
}

func (job *CustomerJob) GetJob(rh *requestHandler, command []string) batchv1.Job {
	return rh.onDemandJob("customer-job", command)
}

func (job *RootJob) GetJob(rh *requestHandler, command []string) batchv1.Job {
	rootUser := int64(0)
	activeDeadlineSeconds := int64(86400) // 24 hours
	backoffLimit := int32(20)

	rootJob := rh.onDemandJob("root-job", command)
	rootJob.Spec.ActiveDeadlineSeconds = &activeDeadlineSeconds
	rootJob.Spec.BackoffLimit = &backoffLimit
	rootJob.Spec.Template.Spec.SecurityContext = &v1.PodSecurityContext{
		RunAsUser: &rootUser,
	}

	return rootJob
}

func (rh *requestHandler) GetOwnedCrons() (*batchv1b1.CronJobList, error) {
	cronList := &batchv1b1.CronJobList{}
	listOpts := client.InNamespace(rh.site.Namespace).MatchingLabels(
		map[string]string{fn.SiteIdLabel: string(rh.site.Id())},
	)
	if err := rh.reconciler.client.List(context.TODO(), listOpts, cronList); err != nil {
		return nil, err
	}

	return cronList, nil
}

/*
* When a cron is removed from the Site, the corresponding CronJob needs to be
* deleted. Because of how events are triggered, there's no way to know what the
* change to the Site was. So an "Unwanted" cron is one with the SiteID label of
* the site we're looking at that does not appear in the site's cron list.
 */
func (rh *requestHandler) deleteUnwantedCrons() (bool, error) {
	cronList, err := rh.GetOwnedCrons()
	if err != nil {
		return false, err
	}

	desired := rh.site.Spec.Crons

	seen := make(map[string]struct{}, len(desired))
	for _, c := range desired {
		seen[c.Name] = struct{}{}
	}
	for _, c := range cronList.Items {
		if _, ok := seen[c.Name]; !ok {
			bg := client.PropagationPolicy(metav1.DeletePropagationBackground)
			if err := rh.reconciler.client.Delete(context.TODO(), &c, bg); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
}

func (rh *requestHandler) reconcileCronJobs() (requeue bool, err error) {
	if requeue, err = rh.deleteUnwantedCrons(); requeue || err != nil {
		return
	}

	for _, cron := range rh.site.Spec.Crons {
		cronjob := &batchv1b1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: cron.Name, Namespace: rh.env.Namespace}}

		op, err := controllerutil.CreateOrUpdate(context.TODO(), rh.reconciler.client, cronjob, func(existing runtime.Object) error {
			realCronJob := existing.(*batchv1b1.CronJob)
			newCronJob := rh.CustomerCronJob(cron)

			if realCronJob.CreationTimestamp.IsZero() {
				newCronJob.DeepCopyInto(realCronJob)
				rh.reconciler.associateResourceWithController(rh.logger, realCronJob, rh.site)
				return nil
			}

			realCronJob.Labels = newCronJob.Labels

			realCronSpec := &realCronJob.Spec
			newSpec := &newCronJob.Spec

			realCronSpec.Suspend = newSpec.Suspend
			realCronSpec.Schedule = newSpec.Schedule
			realCronSpec.ConcurrencyPolicy = newSpec.ConcurrencyPolicy
			realCronSpec.JobTemplate.Spec.Template.Spec.Containers[0].Command = cron.Command

			realCronSpec.StartingDeadlineSeconds = newSpec.StartingDeadlineSeconds

			return nil
		})

		if err != nil {
			return false, err
		}
		if op != controllerutil.OperationResultNone {
			rh.logger.Info("Successfully reconciled CronJob", "Name", cron.Name, "operation", op)
			return true, nil
		}
	}

	return false, nil
}

func getNextJobBuilder(annotations map[string]string) (JobBuilder, string) {
	jobs := []JobBuilder{
		&CustomerJob{},
		&RootJob{},
	}

	// Return first job in annotations
	for key, value := range annotations {
		for _, j := range jobs {
			if key == j.Label() {
				return j, value
			}
		}
	}

	return nil, ""
}

func (rh *requestHandler) reconcileJobs() (requeue bool, err error) {
	annotations := rh.site.GetAnnotations()
	builder, cmdStr := getNextJobBuilder(annotations)
	if builder == nil {
		return false, nil
	}

	cmd := &[]string{}
	if err := yaml.Unmarshal([]byte(cmdStr), cmd); err != nil {
		return false, err
	}

	jobObj := builder.GetJob(rh, *cmd)
	rh.reconciler.associateResourceWithController(rh.logger, &jobObj, rh.site)

	rh.logger.Info("Creating job", "name", jobObj.GetName(), "command", cmdStr)
	err = rh.reconciler.client.Create(context.TODO(), &jobObj)
	if err != nil {
		return false, err
	}

	delete(annotations, builder.Label())
	rh.site.SetAnnotations(annotations)
	if err := rh.reconciler.client.Update(context.TODO(), rh.site); err != nil {
		return false, err
	}

	return true, nil

}
