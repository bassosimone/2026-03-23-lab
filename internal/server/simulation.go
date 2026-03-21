// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bassosimone/2026-03-23-lab/internal/vis"
	"github.com/bassosimone/iss"
)

// Simulation is a simulated internet with DPI-capable routing.
//
// Use [NewSimulation] to construct.
type Simulation struct {
	// dpi is the DPI engine for adding/clearing rules.
	dpi *vis.DPIEngine

	// sim is the underlying [*iss.Simulation].
	sim *iss.Simulation
}

// NewSimulation creates a [*Simulation] with 10ms base propagation
// delay and an active (but empty) DPI engine.
//
// The simulation starts routing packets immediately. Cancel ctx
// and call [*Simulation.Wait] for clean shutdown.
func NewSimulation(ctx context.Context, datadir string, scenario *iss.Scenario) *Simulation {
	dpi := vis.NewDPIEngine()
	router := vis.NewRouter(
		vis.RouterOptionDelay(10*time.Millisecond),
		vis.RouterOptionDPI(dpi),
	)
	return &Simulation{
		dpi: dpi,
		sim: iss.MustNewSimulation(ctx, datadir, scenario, router),
	}
}

// DPI returns the [*vis.DPIEngine] for adding or clearing rules.
func (s *Simulation) DPI() *vis.DPIEngine {
	return s.dpi
}

// ISS returns the underlying [*iss.Simulation] for accessing
// client operations (DialContext, LookupHost, CertPool, etc.).
func (s *Simulation) ISS() *iss.Simulation {
	return s.sim
}

// Wait waits for the simulation to shut down. Cancel the context
// passed to [NewSimulation] before calling this method.
func (s *Simulation) Wait() {
	s.sim.Wait()
}

// RunCommandParams contains the parameters for [*Simulation.RunCommand].
type RunCommandParams struct {
	// Argv is the command line to execute.
	Argv []string

	// Stdout is the standard output writer.
	Stdout io.Writer

	// Stderr is the standard error writer.
	Stderr io.Writer
}

// ErrRunCommandNoCommand indicates that [RunCommandParams.Argv] is empty.
var ErrRunCommandNoCommand = errors.New("server: run: no command specified")

// ErrRunCommandUnknownCommand indicates that the command is not recognized.
var ErrRunCommandUnknownCommand = errors.New("server: run: unknown command")

// RunCommand runs a command within the simulation.
func (s *Simulation) RunCommand(ctx context.Context, params *RunCommandParams) error {
	if len(params.Argv) < 1 {
		return ErrRunCommandNoCommand
	}

	switch params.Argv[0] {
	case "curl":
		return s.runCurl(ctx, params)

	case "dig":
		return s.runDig(ctx, params)

	case "host":
		return s.runHost(ctx, params)

	default:
		return fmt.Errorf("%w: %s", ErrRunCommandUnknownCommand, params.Argv[0])
	}
}
