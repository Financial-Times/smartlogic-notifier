# UPP - Smartlogic Notifier

Entrypoint for concept publish notifications from the Smartlogic Semaphore system.

## Code

smartlogic-notifier

## Primary URL

<https://github.com/Financial-Times/smartlogic-notifier>

## Service Tier

Bronze

## Lifecycle Stage

Production

## Delivered By

content

## Supported By

content

## Known About By

- elitsa.pavlova
- kalin.arsov
- ivan.nikolov
- miroslav.gatsanoga
- dimitar.terziev

## Host Platform

AWS

## Architecture

See the project README for details: <https://github.com/Financial-Times/smartlogic-notifier>

## Contains Personal Data

No

## Contains Sensitive Data

No

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

FullyAutomated

## Failover Details

NotApplicable

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

Manual

## Key Management Process Type

NotApplicable

## Key Management Details

There is no key rotation procedure for this system.

## Monitoring

Look for the pods in the cluster health endpoint and click to see pod health and checks:

- <https://upp-prod-publish-eu.upp.ft.com/__health/>
- <https://upp-prod-publish-us.upp.ft.com/__health/>

## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
