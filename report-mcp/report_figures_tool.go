package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type reportFiguresInput struct {
	SessionID  string `json:"session_id"`
	BundleFile string `json:"bundle_file"`
}
type reportFiguresResult struct {
	SessionID string         `json:"session_id"`
	JobID     string         `json:"job_id"`
	Receipt   figuresReceipt `json:"receipt"`
}

func (a *app) renderReportFigures(ctx context.Context, in reportFiguresInput) (reportFiguresResult, error) {
	var out reportFiguresResult
	if !sessionPattern.MatchString(in.SessionID) || validRel(in.BundleFile) != nil {
		return out, errors.New("FIGURE_BUNDLE_MISMATCH")
	}
	select {
	case a.busy <- struct{}{}:
		defer func() { <-a.busy }()
	default:
		return out, errors.New("FIGURE_RENDER_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	s, e := a.sessions.OpenRoot(in.SessionID)
	if e != nil {
		return out, errors.New("FIGURE_BUNDLE_MISMATCH")
	}
	defer s.Close()
	b, e := boundedRead(s, in.BundleFile, 16<<20)
	if e != nil {
		return out, errors.New("FIGURE_BUNDLE_MISMATCH")
	}
	var bundle svgBundle
	if json.Unmarshal(b, &bundle) != nil || bundle.Version != "customer-visuals-1" || !figureHashRE.MatchString(bundle.ModelSHA256) || bundle.LayoutVersion != "artex-layout-r3b-v1" || len(bundle.Figures) < 1 || len(bundle.Figures) > 50 {
		return out, errors.New("FIGURE_BUNDLE_MISMATCH")
	}
	for _, f := range bundle.Figures {
		if validateFigure(f) != nil {
			return out, errors.New("FIGURE_BUNDLE_MISMATCH")
		}
	}
	regular := os.Getenv("REPORT_FIGURE_FONT_REGULAR")
	mono := os.Getenv("REPORT_FIGURE_FONT_MONO")
	medium := os.Getenv("REPORT_FIGURE_FONT_MEDIUM")
	bold := os.Getenv("REPORT_FIGURE_FONT_BOLD")
	if regular == "" || mono == "" || medium == "" || bold == "" {
		return out, errors.New("FIGURE_RENDER_UNAVAILABLE")
	}
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return out, e
	}
	out.SessionID = in.SessionID
	out.JobID = "job-" + hex.EncodeToString(nonce[:])
	rel := filepath.Join(in.SessionID, out.JobID)
	if e = a.output.MkdirAll(rel, 0750); e != nil {
		return out, e
	}
	dir := filepath.Join(a.cfg.output, rel, "figures")
	r, e := renderFigureBundle(ctx, bundle, dir, env("REPORT_FIGURE_CHROME", "google-chrome"), regular, mono, medium, bold)
	if e != nil {
		return out, errors.New("FIGURE_RENDER_UNAVAILABLE")
	}
	// Keep full local audit files, but avoid duplicating report prose/ledgers in RPC.
	r.DetailTables = nil
	r.Ledger = nil
	out.Receipt = r
	return out, nil
}
