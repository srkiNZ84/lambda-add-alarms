package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func main() {
	fmt.Println("Starting Go AWS client")
	// TODO Make the profile able to be passed in through ENV vars
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("ap-southeast-2"),
		config.WithSharedConfigProfile("AWS-PROFILE-DEV"),
	)

	if err != nil {
		log.Fatalf("Unable to load config, %v", err)
	}
	lambdaSvc := lambda.NewFromConfig(cfg)

	l := getLambdas(lambdaSvc)

	log.Println("List of functions:")
	for _, function := range l {
		log.Printf("Function name is %s", *function.FunctionName)
	}

	// Get list of CloudWatch alarms
	cwSvc := cloudwatch.NewFromConfig(cfg)
	a := getAlarmsMap(cwSvc)

	for a, _ := range a {
		log.Printf("Found Metric alarm %s", a)
	}

	for _, lf := range l {
		if _, exists := a[*lf.FunctionName+"-alarm"]; exists {
			log.Printf("Function %s already has a CloudWatch alarm", *lf.FunctionName)
		} else {
			log.Printf("Function name %s does NOT have a CloudWatch alarm, need to create one", *lf.FunctionName)

			// TODO Need to make sure that the name of the alarm doesn't exceed the AWS max name limit
			alarmName := fmt.Sprintf("%s%s", *lf.FunctionName, "-alarm")
			ep := int32(2)
			d := fmt.Sprintf("Automatically generated alarm for %s function", *lf.FunctionName)
			dk := "FunctionName"
			m := "Errors"
			ns := "AWS/Lambda"
			p := int32(120)
			t := float64(5)
			tk := "Environment"
			tv := "Dev"
			pai := &cloudwatch.PutMetricAlarmInput{
				AlarmName:          &alarmName,
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  &ep,
				AlarmActions:       []string{"arn:aws:sns:ap-southeast-2:1111111111111111111:foo-bar-dev-alarms"},
				AlarmDescription:   &d,
				Dimensions:         []cwtypes.Dimension{{Name: &dk, Value: lf.FunctionName}},
				MetricName:         &m,
				Namespace:          &ns,
				Period:             &p,
				Statistic:          cwtypes.StatisticSum,
				Tags:               []cwtypes.Tag{{Key: &tk, Value: &tv}},
				Threshold:          &t,
			}
			_, err := cwSvc.PutMetricAlarm(context.TODO(), pai)
			if err != nil {
				log.Fatalf("Unable to create alarm for function %s. \nError: %v", *lf.FunctionName, err)

			}

		}
	}

	// TODO For each lambda function check if alarm exists
	// TODO If no alarm exists, create one
}

func getAlarmsMap(c *cloudwatch.Client) map[string]bool {
	a := getAlarms(c)
	am := make(map[string]bool)
	for _, ma := range a {
		am[*ma.AlarmName] = true
	}
	return am
}

func getAlarms(c *cloudwatch.Client) []cwtypes.MetricAlarm {
	// TODO Add pagination to make sure we get the full list
	a, err := c.DescribeAlarms(context.TODO(), &cloudwatch.DescribeAlarmsInput{})
	if err != nil {
		log.Fatalf("Unable to describe alarms, %v", err)
	}
	return a.MetricAlarms
}

func getLambdas(l *lambda.Client) []types.FunctionConfiguration {
	// TODO Add pagination to get the full list for sure
	// TODO Add optional argument to get only functions with a particular tag (e.g. Environment = DEV)
	response, err := l.ListFunctions(context.TODO(), &lambda.ListFunctionsInput{})
	if err != nil {
		log.Fatalf("Unable to list functions, %v", err)
	}

	return response.Functions
}
