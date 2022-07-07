# UPP - Smartlogic Notifier

Entrypoint for concept publish notifications from the Smartlogic Semaphore system.

## Code

smartlogic-notifier

## Primary URL

<https://upp-prod-publish-glb.upp.ft.com/__smartlogic-notifier/>

## Service Tier

Bronze

## Lifecycle Stage

Production

## Host Platform

AWS

## Architecture

The service is deployed in both EU and US regions of UPP Publishing clusters with two replicas per deployment.
There are two separate deployments in each region, one processing updates from the Smartlogic Ontology model and one for the Managed Locations model.

Further you could review the project code: <https://github.com/Financial-Times/smartlogic-notifier>

## Contains Personal Data

No

## Contains Sensitive Data

No

## Failover Architecture Type

ActivePassive

## Failover Process Type

FullyAutomated

## Failback Process Type

PartiallyAutomated

## Failover Details

See the [failover guide](https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/publishing-cluster) for more details.

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

The release is triggered by making a Github release which is then picked up by a Jenkins multibranch pipeline. The Jenkins pipeline should be manually started in order for it to deploy the helm package to the Kubernetes clusters.

## Key Management Process Type

NotApplicable

## Key Management Details

There is no key rotation procedure for this system.

## Monitoring

Look for the pods in the cluster health endpoint and click to see pod health and checks:

- <https://upp-prod-publish-eu.upp.ft.com/__health/__pods-health?service-name=smartlogic-notifier>
- <https://upp-prod-publish-us.upp.ft.com/__health/__pods-health?service-name=smartlogic-notifier>


## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
