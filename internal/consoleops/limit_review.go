package consoleops

import (
	"bytes"
	"context"

	"github.com/hoverwinter/infinity-marketd/internal/ingest"
	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/querier"
)

func LimitCorrectionImporter(store ingest.LimitReviewWriter, timezone string) querier.LimitCorrectionImporter {
	return func(ctx context.Context, payload []byte, dryRun bool) (querier.LimitCorrectionImportResult, error) {
		opts := ingest.LimitReviewImportOptions{DryRun: true, Timezone: timezone}
		if repo, ok := store.(interface {
			LimitEvents(context.Context, querier.LimitQuery) (querier.LimitResult[querier.LimitEvent], error)
		}); ok {
			opts.LoadEvents = func(ctx context.Context, day string) ([]model.LimitEvent, error) {
				return querier.LoadLimitEventFacts(ctx, repo, day)
			}
		}
		summary, err := ingest.ImportLimitReviewCorrectionsReader(ctx, bytes.NewReader(payload), opts)
		if err != nil {
			return correctionResult(summary), querier.ValidationError{Message: err.Error()}
		}
		if dryRun {
			return correctionResult(summary), nil
		}
		opts.DryRun, opts.Store = false, store
		summary, err = ingest.ImportLimitReviewCorrectionsReader(ctx, bytes.NewReader(payload), opts)
		return correctionResult(summary), err
	}
}

func correctionResult(summary ingest.LimitReviewImportSummary) querier.LimitCorrectionImportResult {
	return querier.LimitCorrectionImportResult{RunID: summary.RunID, Events: summary.Events, RowsWritten: summary.RowsWritten, RowsSkipped: summary.RowsSkipped, Issues: nonNilIssues(summary.Issues), DryRun: summary.DryRun}
}
