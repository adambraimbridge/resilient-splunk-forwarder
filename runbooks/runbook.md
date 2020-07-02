<!--
    Written in the format prescribed by https://github.com/Financial-Times/runbook.md.
    Any future edits should abide by this format.
-->

# UPP - Resilient Splunk Forwarder

Forwards logs cached in S3 to Splunk

## Code

resilient-splunk-forwarder

## Primary URL

<https://upp-prod-delivery-glb.upp.ft.com/__resilient-splunk-forwarder/>
<https://upp-prod-publish-glb.upp.ft.com/__resilient-splunk-forwarder/>

## Service Tier

Platinum

## Lifecycle Stage

Production

## Delivered By

content

## Supported By

content

## Known About By

- mihail.mihaylov
- hristo.georgiev
- elitsa.pavlova
- kalin.arsov
- elina.kaneva
- georgi.ivanov

## Host Platform

AWS

## Architecture

The service reads and deletes objects (logs) from S3 and forwards them to the provided Splunk HEC URL. Logs are immediately deleted from S3. Logs are then dispatched to a set of workers that submit the data to the configured Splunk HEC URL. Failed logs are stored again in S3. Failures also cause exponential backoff so that the endopint is not overwhelmed. However, due to having multiple workers, this will not affect logs that are already dispatched.

## Contains Personal Data

No

## Contains Sensitive Data

No

## Dependencies

- splunkcloud

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

FullyAutomated

## Failover Details

The service is deployed in all clusters. The failover guide for the clusters is located here: <https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/>

## Data Recovery Process Type

FullyAutomated

## Data Recovery Details

Data are stored in S3 and are persistent and not dependant on service failire.

## Release Process Type

FullyAutomated

## Rollback Process Type

Manual

## Release Details

The deployment is automated.

## Key Management Process Type

None

## Key Management Details

The service is not using AWS keys but IAM role to access the S3 bucket

## Monitoring

Splunk logs:  
[Logs from k8s PROD from the last hour](https://financialtimes.splunkcloud.com/en-US/app/financial_times_production/search?q=search%20index%3Dcontent_prod%20environment%3D%22upp-prod*%22&sid=1593615745.4269268&display.page.search.mode=verbose&dispatch.sample_ratio=1&earliest=-1h&latest=now)

Check that each environment has ingested logs.  
Grafana statistics:  
<http://grafana.ft.com/dashboard/db/upp-k8s-resilient-splunk-forwarder-stats?orgId=1>

Check for Requests/Sec rates and Splunk HEC (Http Event Collector) errors.  

S3 prod bucket:  
<https://s3.console.aws.amazon.com/s3/buckets/splunklogs-upp-prod/?region=eu-west-1&amp;tab=overview>   
Each environment has its specific folder inside the bucket. These folders should not contain any data (or data size should continuously decrease) if the resilient-splunk-forwarder is successfully ingesting into Splunk.

## First Line Troubleshooting

1. Check the health of the service either on the [C&M Heimdall](https://heimdall.ftops.tech/dashboard?teamid=content) or by calling the __health endpoint.

NOTE: This will require Basic UPP Kubernetes authentication for Ops

<https://upp-prod-delivery-eu.upp.ft.com/__resilient-splunk-forwarder/__health>

<https://upp-prod-delivery-eu.upp.ft.com/__resilient-splunk-forwarder/__health>

<https://upp-prod-publish-us.upp.ft.com/__resilient-splunk-forwarder/__health>

<https://upp-prod-publish-us.upp.ft.com/__resilient-splunk-forwarder/__health>

Available checks reveal the status of S3 and Splunk connectivity.  

2. Check the [log collector](https://biz-ops.in.ft.com/System/log-collector#troubleshooting) for any issues in storing log messages in S3.

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
