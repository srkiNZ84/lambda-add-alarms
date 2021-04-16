package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func main() {
	awsProfile := flag.String("profile", "", "The AWS profile to use when querying Lambdas and creating CloudWatch alarms.")
	devEnvName := flag.String("devenv", "dev", "The name of the environment to use for tags and SNS topics. Defaults to 'dev'.")
	awsRegion := flag.String("region", "ap-southeast-2", "The name of the AWS Region to search for Lambdas and CloudWatch alarms in")

	flag.Parse()

	if *awsProfile == "" {
		log.Fatalf("AWS Profile needs to be set using the '-profile' option")
	}

	fmt.Println("Starting Go AWS client, using profile", *awsProfile)
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(*awsRegion),
		config.WithSharedConfigProfile(*awsProfile),
	)

	if err != nil {
		log.Fatalf("Unable to load AWS config, %v", err)
	}

	stsSvc := sts.NewFromConfig(cfg)
	awsId, err := stsSvc.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("Unable to get the AWS account ID, %v", err)
	}

	lambdaSvc := lambda.NewFromConfig(cfg)

	l := getLambdas(lambdaSvc, *devEnvName)

	log.Printf("Found %v Lambda functions", len(l))

	// Get list of CloudWatch alarms
	cwSvc := cloudwatch.NewFromConfig(cfg)
	cwa := getAlarmsMap(cwSvc)

	log.Printf("Found %v CloudWatch Metric alarms", len(cwa))

	for _, lf := range l {
		if _, exists := cwa[*lf.FunctionName+"-errors-alarm"]; exists {
			log.Printf("Function %s already has a CloudWatch alarm, skipping", *lf.FunctionName)
		} else {
			log.Printf("Function name %s does NOT have a CloudWatch alarm, need to create one...", *lf.FunctionName)

			// TODO Need to make sure that the name of the alarm doesn't exceed the AWS max name limit
			alarmName := fmt.Sprintf("%s%s", *lf.FunctionName, "-errors-alarm")
			ep := int32(1)
			d := fmt.Sprintf("Automatically generated alarm for %s function errors", *lf.FunctionName)
			dk := "FunctionName"
			m := "Errors"
			ns := "AWS/Lambda"
			p := int32(60)
			t := float64(1)
			tk := "Environment"
			tkcb := "CreatedBy"
			tvcb := "lambda-add-alarms"
			pai := &cloudwatch.PutMetricAlarmInput{
				AlarmName:          &alarmName,
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  &ep,
				AlarmActions:       []string{"arn:aws:sns:" + *awsRegion + ":" + *awsId.Account + ":" + *devEnvName + "-advice-online-alarms"},
				AlarmDescription:   &d,
				Dimensions:         []cwtypes.Dimension{{Name: &dk, Value: lf.FunctionName}},
				MetricName:         &m,
				Namespace:          &ns,
				Period:             &p,
				Statistic:          cwtypes.StatisticSum,
				Tags:               []cwtypes.Tag{{Key: &tk, Value: devEnvName}, {Key: &tkcb, Value: &tvcb}},
				Threshold:          &t,
			}
			_, err := cwSvc.PutMetricAlarm(context.TODO(), pai)
			if err != nil {
				log.Fatalf("Unable to create alarm for function %s. \nError: %v", *lf.FunctionName, err)

			}
		}
	}
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
	cwa := []cwtypes.MetricAlarm{}
	ap := cloudwatch.NewDescribeAlarmsPaginator(c, &cloudwatch.DescribeAlarmsInput{})
	for ap.HasMorePages() {
		a, err := ap.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("Unable to describe alarms, %v", err)
		}
		cwa = append(cwa, a.MetricAlarms...)
	}
	return cwa
}

func getLambdas(l *lambda.Client, envTag string) []types.FunctionConfiguration {
	lf := []types.FunctionConfiguration{}
	lp := lambda.NewListFunctionsPaginator(l, &lambda.ListFunctionsInput{})
	for lp.HasMorePages() {
		response, err := lp.NextPage(context.TODO())
		if err != nil {
			log.Fatalf("Unable to list functions, %v", err)
		}
		for _, lamFunc := range response.Functions {
			ts, err := l.ListTags(context.TODO(), &lambda.ListTagsInput{Resource: lamFunc.FunctionArn})
			if err != nil {
				log.Fatalf("Unable to get tags for function %s, error: %v", *lamFunc.FunctionArn, err)
			}
			for tn, v := range ts.Tags {
				if tn == "STAGE" && v == envTag {
					//log.Printf("Adding function %s because tag %s with value %s matches", *lamFunc.FunctionName, tn, v)
					lf = append(lf, lamFunc)
				}
			}
		}
	}
	return lf
}
