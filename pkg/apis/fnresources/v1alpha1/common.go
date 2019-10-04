package v1alpha1

const (
	LabelPrefix        = "fnresources.acquia.io/"
	ApplicationIdLabel = LabelPrefix + "application-id"
	EnvironmentIdLabel = LabelPrefix + "environment-id"
	SiteIdLabel        = LabelPrefix + "site-id"
	GitRepoLabel       = LabelPrefix + "git-repo"
	GitRefLabel        = LabelPrefix + "git-ref"

	DomainMapName = "domain-map"
)
