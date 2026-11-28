package server

import (
	"context"
	"log/slog"
	"time"

	"app-suite/internal/db"
	"app-suite/internal/gpose"
)

// A gpose staff upload for a completed sign-up is kept a day (gpose.PhotoLifetime).
// The API stops answering it the moment its day is over; this sweeper then deletes its
// file and clears it from its sign-up, so nothing of it outlives the day on disk.

// gposePhotoSchedulerInterval is how often the sweeper looks for gposes whose day is
// over: the longest a file can stay on disk past it. The read is one indexed query.
const gposePhotoSchedulerInterval = time.Minute

// RunGposePhotoScheduler deletes the gposes whose day is over, until ctx is cancelled.
// Safe to call in a goroutine.
func (s *Server) RunGposePhotoScheduler(ctx context.Context) {
	runScheduler(ctx, "gpose-photos", gposePhotoSchedulerInterval, func() { s.expireGposePhotos(ctx, time.Now()) })
}

// expireGposePhotos deletes every gpose whose day is over at now, then any file in the
// folder that outlived its day or that no sign-up names (an upload whose sign-up never
// recorded it). It runs on startup too, so a day that ended while the process was down
// is swept as soon as it is back.
func (s *Server) expireGposePhotos(ctx context.Context, now time.Time) {
	due, err := db.New(s.store.ReadDB()).ListExpiredGposePhotos(ctx, now.UTC().Format(time.RFC3339))
	if err != nil {
		slog.Log(ctx, errorLevel(err), "gpose photo sweep: load expired photos", "error", err)
		return
	}
	cleared := 0
	for _, p := range due {
		// Named by token, so one staff replaced since the read is left alone.
		n, err := db.New(s.store.DB()).ClearGposeSignupPhoto(ctx, db.ClearGposeSignupPhotoParams{
			ID: p.ID, PhotoToken: p.PhotoToken,
		})
		if err != nil {
			slog.Error("gpose photo sweep: clear a photo", "signupId", p.ID, "error", err)
			continue
		}
		s.gposePhotos.Remove(p.PhotoToken)
		if n > 0 {
			cleared++
		}
	}
	if cleared > 0 {
		slog.Info("gpose photos expired", "count", cleared)
		// No request made this happen, so the Gpose Booth pages are told here.
		s.resourceChanged("gpose-booths")
	}

	tokens, err := db.New(s.store.ReadDB()).ListGposePhotoTokens(ctx)
	if err != nil {
		slog.Log(ctx, errorLevel(err), "gpose photo sweep: load held photos", "error", err)
		return
	}
	held := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		held[t] = true
	}
	pruned, err := s.gposePhotos.Prune(held, gpose.PhotoLifetime, now)
	if err != nil {
		slog.Warn("gpose photo sweep: read the folder", "error", err)
		return
	}
	if pruned > 0 {
		slog.Info("gpose photo files pruned", "count", pruned)
	}
}
