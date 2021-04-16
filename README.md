# Lambda Add Alarms

## About

Go program that takes an AWS profile and a environment name (i.e. AWS Tag) and checks for any Lambdas that do not have CloudWatch alerts set on their "Errors" metric.

Any Lambdas that it finds that have no alerts, it creates alerts for. The alerts trigger any time that a Lambda function records an "error".

## How to use

TODO

## How does it work

TODO

## Build

TODO

## Things left to build and fix

* Add checking to make sure we're not exceeding the maximum CW Alarm name
* Add unit tests to everything
* Make the environment "tag key" configurable
* Make the SNS topic configurable