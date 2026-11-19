package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/reels"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const reelsQueue = "reels"

// ReelBuildArgs builds one Facebook Reel draft. Without a reel id it is the scheduled run, which
// picks a trending storyline itself; Rerender redraws an existing draft from its saved script.
type ReelBuildArgs struct {
	ReelID   string `json:"reel_id,omitempty"`
	Rerender bool   `json:"rerender,omitempty"`
}

func (ReelBuildArgs) Kind() string { return "reels.build" }

// One attempt only: a build spends several model and image calls, and failures are recorded on the draft.
func (ReelBuildArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: reelsQueue, MaxAttempts: 1, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
	}}}
}

type ReelBuildWorker struct {
	river.WorkerDefaults[ReelBuildArgs]
	Reels *reels.Service
}

func (worker *ReelBuildWorker) Work(ctx context.Context, job *river.Job[ReelBuildArgs]) error {
	if worker.Reels == nil {
		return nil
	}
	return worker.Reels.Run(ctx, job.Args.ReelID, job.Args.Rerender)
}

func (worker *ReelBuildWorker) Timeout(*river.Job[ReelBuildArgs]) time.Duration {
	return 30 * time.Minute
}

func EnqueueReelBuild(ctx context.Context, client *river.Client[pgx.Tx], reelID string, rerender bool) error {
	_, err := client.Insert(ctx, ReelBuildArgs{ReelID: reelID, Rerender: rerender}, nil)
	return err
}

// hoursSchedule fires at the given local hours every day.
type hoursSchedule struct {
	location *time.Location
	hours    []int
}

func (schedule hoursSchedule) Next(current time.Time) time.Time {
	local := current.In(schedule.location)
	for day := 0; day < 2; day++ {
		for _, hour := range schedule.hours {
			next := time.Date(local.Year(), local.Month(), local.Day()+day, hour, 0, 0, 0, schedule.location)
			if next.After(local) {
				return next
			}
		}
	}
	return local.Add(24 * time.Hour)
}

// reelSchedule drafts a reel every six hours on Colombo time: 00:00, 06:00, 12:00 and 18:00.
func reelSchedule() river.PeriodicSchedule {
	location, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		location = time.FixedZone("Asia/Colombo", 5*3600+1800)
	}
	return hoursSchedule{location: location, hours: []int{0, 6, 12, 18}}
}
