package ops

import (
	"github.com/ess/dry"

	"github.com/starkandwayne/scheduler-for-ocf/core"
)

func ExecuteJob(raw dry.Value) dry.Result {
	input := core.Inputify(raw)
	services := input.Services
	job, _ := input.Executable.ToJob()
	execution := input.Execution

	services.Runner.Execute(services, execution, job)

	return dry.Success(input)
}
