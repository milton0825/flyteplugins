package bigquery

import (
	"context"
	"encoding/gob"
	"fmt"
	"github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/google"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"golang.org/x/oauth2"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/googleapi"
	"net/http"
	"time"

	flyteIdlCore "github.com/flyteorg/flyteidl/gen/pb-go/flyteidl/core"
	pluginErrors "github.com/flyteorg/flyteplugins/go/tasks/errors"
	pluginsCore "github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/core"
	"google.golang.org/api/option"

	"github.com/flyteorg/flytestdlib/logger"

	"github.com/flyteorg/flytestdlib/promutils"

	"github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery"
	"github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/core"
	"github.com/flyteorg/flyteplugins/go/tasks/pluginmachinery/webapi"
)

const (
	bigqueryQueryJobTask = "bigquery_query_job_task"
)

type Plugin struct {
	metricScope       promutils.Scope
	cfg               *Config
	googleTokenSource google.TokenSource
}

type ResourceWrapper struct {
	Status      *bigquery.JobStatus
	CreateError *googleapi.Error
}

type ResourceMetaWrapper struct {
	K8sServiceAccount string
	Namespace         string
	JobReference      bigquery.JobReference
}

func (p Plugin) GetConfig() webapi.PluginConfig {
	return GetConfig().WebAPI
}

func (p Plugin) ResourceRequirements(_ context.Context, _ webapi.TaskExecutionContextReader) (
	namespace core.ResourceNamespace, constraints core.ResourceConstraintsSpec, err error) {

	// Resource requirements are assumed to be the same.
	return "default", p.cfg.ResourceConstraints, nil
}

func (p Plugin) Create(ctx context.Context, taskCtx webapi.TaskExecutionContextReader) (webapi.ResourceMeta,
	webapi.Resource, error) {
	return p.createImpl(ctx, taskCtx)
}

func (p Plugin) createImpl(ctx context.Context, taskCtx webapi.TaskExecutionContextReader) (*ResourceMetaWrapper,
	*ResourceWrapper, error) {

	taskTemplate, err := taskCtx.TaskReader().Read(ctx)
	jobId := taskCtx.TaskExecutionMetadata().GetTaskExecutionID().GetGeneratedName()

	if err != nil {
		return nil, nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to fetch task specification")
	}

	inputs, err := taskCtx.InputReader().Get(ctx)

	if err != nil {
		return nil, nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to fetch task inputs")
	}

	var job *bigquery.Job

	namespace := taskCtx.TaskExecutionMetadata().GetNamespace()
	k8sServiceAccount := taskCtx.TaskExecutionMetadata().GetK8sServiceAccount()
	identity := google.Identity{K8sNamespace: namespace, K8sServiceAccount: k8sServiceAccount}
	tokenSource, err := p.googleTokenSource.GetTokenSource(ctx, identity)

	if err != nil {
		return nil, nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to get token source")
	}

	client, err := newBigQueryClient(ctx, tokenSource)

	if err != nil {
		return nil, nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to get bigquery client")
	}

	if taskTemplate.Type == bigqueryQueryJobTask {
		job, err = createQueryJob(jobId, taskTemplate.GetCustom(), inputs)
	} else {
		err = pluginErrors.Errorf(pluginErrors.BadTaskSpecification, "unexpected task type [%v]", taskTemplate.Type)
	}

	if err != nil {
		return nil, nil, err
	}

	job.Configuration.Labels = taskCtx.TaskExecutionMetadata().GetLabels()

	resp, err := client.Jobs.Insert(job.JobReference.ProjectId, job).Do()

	if err != nil {
		apiError, ok := err.(*googleapi.Error)
		resourceMeta := ResourceMetaWrapper{
			JobReference:      *job.JobReference,
			Namespace:         namespace,
			K8sServiceAccount: k8sServiceAccount,
		}

		if ok && apiError.Code == 409 {
			job, err := client.Jobs.Get(resourceMeta.JobReference.ProjectId, resourceMeta.JobReference.JobId).Do()

			if err != nil {
				err := pluginErrors.Wrapf(
					pluginErrors.RuntimeFailure,
					err,
					"failed to get job [%s]",
					formatJobReference(resourceMeta.JobReference))

				return nil, nil, err
			}

			resource := ResourceWrapper{Status: job.Status}

			return &resourceMeta, &resource, nil
		}

		if ok {
			resource := ResourceWrapper{CreateError: apiError}

			return &resourceMeta, &resource, nil
		}

		return nil, nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "failed to create query job")
	}

	resource := ResourceWrapper{Status: resp.Status}
	resourceMeta := ResourceMetaWrapper{
		JobReference:      *job.JobReference,
		Namespace:         namespace,
		K8sServiceAccount: k8sServiceAccount,
	}

	return &resourceMeta, &resource, nil
}

func createQueryJob(jobID string, custom *structpb.Struct, inputs *flyteIdlCore.LiteralMap) (*bigquery.Job, error) {
	queryJobConfig, err := unmarshalQueryJobConfig(custom)

	if err != nil {
		return nil, pluginErrors.Wrapf(pluginErrors.BadTaskSpecification, err, "can't unmarshall struct to QueryJobConfig")
	}

	jobConfigurationQuery, err := getJobConfigurationQuery(queryJobConfig, inputs)

	if err != nil {
		return nil, pluginErrors.Wrapf(pluginErrors.BadTaskSpecification, err, "unable to fetch task inputs")
	}

	jobReference := bigquery.JobReference{
		JobId:     jobID,
		Location:  queryJobConfig.Location,
		ProjectId: queryJobConfig.ProjectID,
	}

	return &bigquery.Job{
		Configuration: &bigquery.JobConfiguration{
			Query: jobConfigurationQuery,
		},
		JobReference: &jobReference,
	}, nil
}

func (p Plugin) Get(ctx context.Context, taskCtx webapi.GetContext) (latest webapi.Resource, err error) {
	return p.getImpl(ctx, taskCtx)
}

func (p Plugin) getImpl(ctx context.Context, taskCtx webapi.GetContext) (wrapper *ResourceWrapper, err error) {
	resourceMeta := taskCtx.ResourceMeta().(*ResourceMetaWrapper)

	tokenSource, err := p.googleTokenSource.GetTokenSource(ctx, google.Identity{
		K8sNamespace:      resourceMeta.Namespace,
		K8sServiceAccount: resourceMeta.K8sServiceAccount,
	})

	if err != nil {
		return nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to get token source")
	}

	client, err := newBigQueryClient(ctx, tokenSource)

	if err != nil {
		return nil, pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to get client")
	}

	job, err := client.Jobs.Get(resourceMeta.JobReference.ProjectId, resourceMeta.JobReference.JobId).Do()

	if err != nil {
		err := pluginErrors.Wrapf(
			pluginErrors.RuntimeFailure,
			err,
			"failed to get job [%s]",
			formatJobReference(resourceMeta.JobReference))

		return nil, err
	}

	return &ResourceWrapper{
		Status: job.Status,
	}, nil
}

func (p Plugin) Delete(ctx context.Context, taskCtx webapi.DeleteContext) error {
	if taskCtx.ResourceMeta() == nil {
		return nil
	}

	resourceMeta := taskCtx.ResourceMeta().(*ResourceMetaWrapper)
	tokenSource, err := p.googleTokenSource.GetTokenSource(ctx, google.Identity{
		K8sNamespace:      resourceMeta.Namespace,
		K8sServiceAccount: resourceMeta.K8sServiceAccount,
	})

	if err != nil {
		return pluginErrors.Wrapf(pluginErrors.RuntimeFailure, err, "unable to get token source")
	}

	client, err := newBigQueryClient(ctx, tokenSource)

	if err != nil {
		return err
	}

	_, err = client.Jobs.Cancel(resourceMeta.JobReference.ProjectId, resourceMeta.JobReference.JobId).Do()

	if err != nil {
		return err
	}

	logger.Info(ctx, "Cancelled job [%s]", formatJobReference(resourceMeta.JobReference))

	return nil
}

func (p Plugin) Status(_ context.Context, tCtx webapi.StatusContext) (phase core.PhaseInfo, err error) {
	resourceMeta := tCtx.ResourceMeta().(*ResourceMetaWrapper)
	resource := tCtx.Resource().(*ResourceWrapper)
	version := pluginsCore.DefaultPhaseVersion

	if resource == nil {
		return core.PhaseInfoUndefined, nil
	}

	taskInfo := createTaskInfo(resourceMeta)

	if resource.CreateError != nil {
		return handleCreateError(resource.CreateError, taskInfo)
	}

	switch resource.Status.State {
	case "PENDING":
		return core.PhaseInfoRunning(version, taskInfo), nil

	case "RUNNING":
		return core.PhaseInfoRunning(version, taskInfo), nil

	case "DONE":
		if resource.Status.ErrorResult != nil {
			return handleErrorResult(
				resource.Status.ErrorResult.Reason,
				resource.Status.ErrorResult.Message,
				taskInfo)
		}

		return pluginsCore.PhaseInfoSuccess(taskInfo), nil
	}

	return core.PhaseInfoUndefined, pluginErrors.Errorf(pluginsCore.SystemErrorCode, "unknown execution phase [%v].", resource.Status.State)
}

func handleCreateError(createError *googleapi.Error, taskInfo *core.TaskInfo) (core.PhaseInfo, error) {
	code := fmt.Sprintf("http%d", createError.Code)

	userExecutionError := &flyteIdlCore.ExecutionError{
		Message: createError.Message,
		Kind:    flyteIdlCore.ExecutionError_USER,
		Code:    code,
	}

	systemExecutionError := &flyteIdlCore.ExecutionError{
		Message: createError.Message,
		Kind:    flyteIdlCore.ExecutionError_SYSTEM,
		Code:    code,
	}

	if createError.Code >= http.StatusBadRequest && createError.Code < http.StatusInternalServerError {
		return core.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil
	}

	if createError.Code >= http.StatusInternalServerError {
		return core.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, systemExecutionError, taskInfo), nil
	}

	// something unexpected happened, just terminate task
	return core.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, systemExecutionError, taskInfo), nil
}

func handleErrorResult(reason string, message string, taskInfo *core.TaskInfo) (core.PhaseInfo, error) {
	userExecutionError := &flyteIdlCore.ExecutionError{
		Message: message,
		Kind:    flyteIdlCore.ExecutionError_USER,
		Code:    reason,
	}

	systemExecutionError := &flyteIdlCore.ExecutionError{
		Message: message,
		Kind:    flyteIdlCore.ExecutionError_SYSTEM,
		Code:    reason,
	}

	// see https://cloud.google.com/bigquery/docs/error-messages

	// user errors are errors where users have to take action, e.g. fix their code
	// all errors with project configuration are also considered as user errors

	// system errors are errors where system doesn't work well and system owners have to take action
	// all errors internal to BigQuery are also considered as system errors

	// transient errors are retryable, if any action is needed, errors are permanent

	switch reason {
	case "":
		return pluginsCore.PhaseInfoSuccess(taskInfo), nil

	// This error returns when you try to access a resource such as a dataset, table, view, or job that you
	// don't have access to. This error also returns when you try to modify a read-only object.
	case "accessDenied":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when there is a temporary server failure such as a network connection problem or
	// a server overload.
	case "backendError":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, systemExecutionError, taskInfo), nil

	// This error returns when billing isn't enabled for the project.
	case "billingNotEnabled":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when BigQuery has temporarily denylisted the operation you attempted to perform,
	// usually to prevent a service outage. This error rarely occurs.
	case "blocked":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when trying to create a job, dataset, or table that already exists. The error also
	// returns when a job's writeDisposition property is set to WRITE_EMPTY and the destination table accessed
	// by the job already exists.
	case "duplicate":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when an internal error occurs within BigQuery.
	case "internalError":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, systemExecutionError, taskInfo), nil

	// This error returns when there is any kind of invalid input other than an invalid query, such as missing
	// required fields or an invalid table schema. Invalid queries return an invalidQuery error instead.
	case "invalid":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when you attempt to run an invalid query.
	case "invalidQuery":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when you attempt to schedule a query with invalid user credentials.
	case "invalidUser":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, systemExecutionError, taskInfo), nil

	// This error returns when you refer to a resource (a dataset, a table, or a job) that doesn't exist.
	// This can also occur when using snapshot decorators to refer to deleted tables that have recently been
	// streamed to.
	case "notFound":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This job error returns when you try to access a feature that isn't implemented.
	case "notImplemented":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when your project exceeds a BigQuery quota, a custom quota, or when you haven't set up
	// billing and you have exceeded the free tier for queries.
	case "quotaExceeded":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, userExecutionError, taskInfo), nil

	case "rateLimitExceeded":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, userExecutionError, taskInfo), nil

	// This error returns when you try to delete a dataset that contains tables or when you try to delete a job
	// that is currently running.
	case "resourceInUse":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, systemExecutionError, taskInfo), nil

	// This error returns when your query uses too many resources.
	case "resourcesExceeded":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This error returns when your query's results are larger than the maximum response size. Some queries execute
	// in multiple stages, and this error returns when any stage returns a response size that is too large, even if
	// the final result is smaller than the maximum. This error commonly returns when queries use an ORDER BY
	// clause.
	case "responseTooLarge":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// This status code returns when a job is canceled.
	case "stopped":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	// Certain BigQuery tables are backed by data managed by other Google product teams. This error indicates that
	// one of these tables is unavailable.
	case "tableUnavailable":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhaseRetryableFailure, systemExecutionError, taskInfo), nil

	// The job timed out.
	case "timeout":
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, userExecutionError, taskInfo), nil

	default:
		return pluginsCore.PhaseInfoFailed(pluginsCore.PhasePermanentFailure, systemExecutionError, taskInfo), nil
	}
}

func createTaskInfo(resourceMeta *ResourceMetaWrapper) *core.TaskInfo {
	timeNow := time.Now()
	j := fmt.Sprintf("bq:%s:%s", resourceMeta.JobReference.Location, resourceMeta.JobReference.JobId)

	return &core.TaskInfo{
		OccurredAt: &timeNow,
		Logs: []*flyteIdlCore.TaskLog{
			{
				Uri: fmt.Sprintf("https://console.cloud.google.com/bigquery?project=%v&j=%v&page=queryresults",
					resourceMeta.JobReference.ProjectId,
					j),
				Name: "BigQuery Console",
			},
		},
	}
}

func formatJobReference(reference bigquery.JobReference) string {
	return fmt.Sprintf("%s:%s.%s", reference.ProjectId, reference.Location, reference.JobId)
}

func newBigQueryClient(ctx context.Context, tokenSource oauth2.TokenSource) (*bigquery.Service, error) {
	options := []option.ClientOption{
		option.WithScopes("https://www.googleapis.com/auth/bigquery"),
		// FIXME how do I access current version?
		option.WithUserAgent(fmt.Sprintf("%s/%s", "flytepropeller", "LATEST")),
		option.WithTokenSource(tokenSource),
	}

	return bigquery.NewService(ctx, options...)
}

func NewPlugin(cfg *Config, metricScope promutils.Scope) (*Plugin, error) {
	googleTokenSource, err := google.NewTokenSource(cfg.GoogleTokenSource)

	if err != nil {
		return nil, pluginErrors.Wrapf(pluginErrors.PluginInitializationFailed, err, "failed to get google token source")
	}

	return &Plugin{
		metricScope:       metricScope,
		cfg:               cfg,
		googleTokenSource: googleTokenSource,
	}, nil
}

func init() {
	gob.Register(ResourceMetaWrapper{})
	gob.Register(ResourceWrapper{})

	pluginmachinery.PluginRegistry().RegisterRemotePlugin(webapi.PluginEntry{
		ID:                 "bigquery",
		SupportedTaskTypes: []core.TaskType{bigqueryQueryJobTask},
		PluginLoader: func(ctx context.Context, iCtx webapi.PluginSetupContext) (webapi.AsyncPlugin, error) {
			return NewPlugin(GetConfig(), iCtx.MetricsScope())
		},
	})
}