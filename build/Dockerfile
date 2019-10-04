FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/fn-drupal-operator \
    USER_UID=1001 \
    USER_NAME=fn-drupal-operator

# install operator binary
COPY build/_output/bin/fn-drupal-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
