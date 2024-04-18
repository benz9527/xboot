#!/bin/bash

# Download fluentbit operator helm chart and install it.
# https://github.com/fluent/fluent-operator/tree/master/charts/fluent-operator

# OCP env, default namespace is fluent.
# The requests.limits.memory is 512Mi, if it is not enough, the 
# fluentbit will be OOM killed by OCP.
helm install fluent-operator ./fluent-operator.tgz -n xxx --set containerRuntime=crio --set operator.logPath.crio=/var/log --set operator.resources.requests.limits.memory=512Mi --set operator.resources.requests.limits.cpu=100m

