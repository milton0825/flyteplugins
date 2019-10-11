package awsbatch

import (
	"context"
	"fmt"

	arrayCore "github.com/lyft/flyteplugins/go/tasks/plugins/array/core"

	"github.com/lyft/flytestdlib/bitarray"

	"github.com/lyft/flyteplugins/go/tasks/plugins/array/arraystatus"
	"github.com/lyft/flyteplugins/go/tasks/plugins/array/awsbatch/config"
	"github.com/lyft/flyteplugins/go/tasks/plugins/array/errorcollector"

	idlCore "github.com/lyft/flyteidl/gen/pb-go/flyteidl/core"
	"github.com/lyft/flyteplugins/go/tasks/pluginmachinery/core"
)

func CheckSubTasksState(ctx context.Context, taskMeta core.TaskExecutionMetadata, jobStore *JobStore,
	cfg *config.Config, currentState *State) (newState *State, err error) {

	logLinks := make([]*idlCore.TaskLog, 0, 4)
	newState = currentState
	parentState := currentState.State

	msg := errorcollector.NewErrorMessageCollector()
	newArrayStatus := arraystatus.ArrayStatus{
		Summary:  arraystatus.ArraySummary{},
		Detailed: arrayCore.NewPhasesCompactArray(uint(currentState.GetExecutionArraySize())),
	}

	jobName := taskMeta.GetTaskExecutionID().GetGeneratedName()
	job := jobStore.Get(jobName)
	// If job isn't currently being monitored (recovering from a restart?), add it to the sync-cache and return
	if job == nil {
		_, err = jobStore.GetOrCreate(jobName, &Job{
			ID:             *currentState.ExternalJobID,
			OwnerReference: taskMeta.GetOwnerID(),
			SubJobs:        make([]*Job, currentState.GetExecutionArraySize()),
		})

		if err != nil {
			return nil, err
		}

		return currentState, nil
	}

	for childIdx, existingPhaseIdx := range currentState.GetArrayStatus().Detailed.GetItems() {
		existingPhase := core.Phases[existingPhaseIdx]
		if existingPhase.IsTerminal() {
			// If we get here it means we have already "processed" this terminal phase since we will only persist
			// the phase after all processing is done (e.g. check outputs/errors file, record events... etc.).
			newArrayStatus.Summary.Inc(existingPhase)
			newArrayStatus.Detailed.SetItem(childIdx, bitarray.Item(existingPhase))

			// TODO: collect log links before doing this
			continue
		}

		subJob := job.SubJobs[childIdx]
		originalIndex := calculateOriginalIndex(childIdx, currentState.GetIndexesToCache())
		logLinks = append(logLinks, &idlCore.TaskLog{
			Name: fmt.Sprintf("AWS Batch Job #%v", originalIndex),
			// TODO: Get job queue
			Uri: GetJobUri(currentState.GetExecutionArraySize(), jobStore.Client.GetAccountID(),
				jobStore.Client.GetRegion(), "", job.ID, childIdx),
		})

		for _, attempt := range subJob.Attempts {
			logLinks = append(logLinks, &idlCore.TaskLog{
				Name: fmt.Sprintf("AWS Batch #%v (%v)", originalIndex, subJob.Status.Phase),
				Uri:  fmt.Sprintf(LogStreamFormatter, jobStore.GetRegion(), attempt.LogStream),
			})
		}

		if subJob.Status.Phase.IsFailure() {
			if len(subJob.Status.Message) > 0 {
				msg.Collect(childIdx, subJob.Status.Message)
			}
		}

		newArrayStatus.Detailed.SetItem(childIdx, bitarray.Item(subJob.Status.Phase))
		newArrayStatus.Summary.Inc(subJob.Status.Phase)
	}

	parentState = parentState.SetArrayStatus(newArrayStatus)

	phase := arrayCore.SummaryToPhase(ctx, currentState.GetOriginalMinSuccesses()-currentState.GetOriginalArraySize()-int64(currentState.GetExecutionArraySize()), newArrayStatus.Summary)
	if phase == arrayCore.PhasePermanentFailure || phase == arrayCore.PhaseRetryableFailure {
		errorMsg := msg.Summary(cfg.MaxErrorStringLength)
		parentState = parentState.SetReason(errorMsg)
	}

	if phase == arrayCore.PhaseCheckingSubTaskExecutions {
		newPhaseVersion := uint32(0)
		if phase == arrayCore.PhaseCheckingSubTaskExecutions {
			// For now, the only changes to PhaseVersion and PreviousSummary occur for running array jobs.
			for phase, count := range parentState.GetArrayStatus().Summary {
				newPhaseVersion += uint32(phase) * uint32(count)
			}
		}

		parentState = parentState.SetPhase(phase, newPhaseVersion)
	} else {
		parentState = parentState.SetPhase(phase, core.DefaultPhaseVersion)
	}

	newState.State = parentState
	return newState, nil
}

// Compute the original index of a sub-task.
func calculateOriginalIndex(childIdx int, toCache *bitarray.BitSet) int {
	var sum = 0
	for i := uint(0); i < toCache.Cap(); i++ {
		if !toCache.IsSet(i) {
			if sum == childIdx {
				return int(i)
			}

			sum++
		}
	}

	return -1
}
