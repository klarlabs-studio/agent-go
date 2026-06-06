// Package testing provides test automation tools for agent-go.
//
// The pack uses an interface-based approach, allowing any test runner
// (go test, pytest, jest, etc.) to be plugged in.
package testing

import (
	"context"
	"encoding/json"
	"fmt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// TestRunner provides test execution capabilities.
type TestRunner interface {
	// RunTests executes tests matching the given pattern.
	RunTests(ctx context.Context, opts RunOptions) (*TestReport, error)

	// GetCoverage generates a coverage report.
	GetCoverage(ctx context.Context, opts CoverageOptions) (*CoverageReport, error)

	// RunBenchmark executes benchmark tests.
	RunBenchmark(ctx context.Context, opts BenchmarkOptions) (*BenchmarkReport, error)
}

// TestGenerator generates test code.
type TestGenerator interface {
	// GenerateTest generates a test for the given source code.
	GenerateTest(ctx context.Context, source string, language string, opts GenerateOptions) (*GeneratedTest, error)
}

// MutationTester performs mutation testing.
type MutationTester interface {
	// MutationTest runs mutation testing on the given source.
	MutationTest(ctx context.Context, opts MutationOptions) (*MutationReport, error)
}

// RunOptions configures test execution.
type RunOptions struct {
	Pattern  string   `json:"pattern,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Verbose  bool     `json:"verbose,omitempty"`
	Timeout  string   `json:"timeout,omitempty"`
	Race     bool     `json:"race,omitempty"`
	Parallel int      `json:"parallel,omitempty"`
}

// TestReport contains test execution results.
type TestReport struct {
	Total    int          `json:"total"`
	Passed   int          `json:"passed"`
	Failed   int          `json:"failed"`
	Skipped  int          `json:"skipped"`
	Duration string       `json:"duration"`
	Tests    []TestResult `json:"tests,omitempty"`
}

// TestResult represents a single test result.
type TestResult struct {
	Name     string `json:"name"`
	Package  string `json:"package,omitempty"`
	Status   string `json:"status"` // "pass", "fail", "skip"
	Duration string `json:"duration,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// CoverageOptions configures coverage collection.
type CoverageOptions struct {
	Pattern    string `json:"pattern,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
	Format     string `json:"format,omitempty"` // "text", "html", "json"
}

// CoverageReport contains coverage results.
type CoverageReport struct {
	TotalCoverage float64           `json:"total_coverage"`
	Packages      []PackageCoverage `json:"packages,omitempty"`
	Uncovered     []UncoveredBlock  `json:"uncovered,omitempty"`
}

// PackageCoverage represents coverage for a single package.
type PackageCoverage struct {
	Name       string  `json:"name"`
	Coverage   float64 `json:"coverage"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
}

// UncoveredBlock represents an uncovered code block.
type UncoveredBlock struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// BenchmarkOptions configures benchmark execution.
type BenchmarkOptions struct {
	Pattern string `json:"pattern,omitempty"`
	Count   int    `json:"count,omitempty"`
	Time    string `json:"time,omitempty"`
}

// BenchmarkReport contains benchmark results.
type BenchmarkReport struct {
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

// BenchmarkResult represents a single benchmark result.
type BenchmarkResult struct {
	Name        string  `json:"name"`
	Iterations  int     `json:"iterations"`
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  int64   `json:"bytes_per_op,omitempty"`
	AllocsPerOp int64   `json:"allocs_per_op,omitempty"`
}

// GenerateOptions configures test generation.
type GenerateOptions struct {
	Style     string `json:"style,omitempty"` // "table", "subtest", "basic"
	Framework string `json:"framework,omitempty"`
}

// GeneratedTest contains generated test code.
type GeneratedTest struct {
	Code     string `json:"code"`
	FileName string `json:"file_name"`
	Language string `json:"language"`
}

// MutationOptions configures mutation testing.
type MutationOptions struct {
	Pattern  string   `json:"pattern,omitempty"`
	Mutators []string `json:"mutators,omitempty"`
	Timeout  string   `json:"timeout,omitempty"`
}

// MutationReport contains mutation testing results.
type MutationReport struct {
	TotalMutants int            `json:"total_mutants"`
	Killed       int            `json:"killed"`
	Survived     int            `json:"survived"`
	Timeout      int            `json:"timeout"`
	Score        float64        `json:"score"`
	Mutants      []MutantResult `json:"mutants,omitempty"`
}

// MutantResult represents a single mutation test result.
type MutantResult struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Mutator  string `json:"mutator"`
	Status   string `json:"status"` // "killed", "survived", "timeout"
	Original string `json:"original,omitempty"`
	Mutated  string `json:"mutated,omitempty"`
}

// Config holds testing pack configuration.
type Config struct {
	// Runner is the test runner (required).
	Runner TestRunner

	// Generator is an optional test code generator.
	Generator TestGenerator

	// Mutator is an optional mutation tester.
	Mutator MutationTester
}

// Pack returns the test automation tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &testingPack{cfg: cfg}

	tools := []tool.Tool{
		p.runTestsTool(),
		p.coverageReportTool(),
		p.benchmarkTool(),
	}

	if cfg.Generator != nil {
		tools = append(tools, p.generateTestTool())
	}
	if cfg.Mutator != nil {
		tools = append(tools, p.mutationTestTool())
	}

	return pack.NewBuilder("testing").
		WithDescription("Test automation tools: run tests, coverage reports, benchmarks, test generation, mutation testing").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		AllowAllInState(agent.StateValidate).
		Build()
}

type testingPack struct {
	cfg Config
}

func (p *testingPack) runTestsTool() tool.Tool {
	return tool.NewBuilder("test_run_tests").
		WithDescription("Run tests matching a pattern").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pattern  string   `json:"pattern,omitempty"`
				Tags     []string `json:"tags,omitempty"`
				Verbose  bool     `json:"verbose,omitempty"`
				Timeout  string   `json:"timeout,omitempty"`
				Race     bool     `json:"race,omitempty"`
				Parallel int      `json:"parallel,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			report, err := p.cfg.Runner.RunTests(ctx, RunOptions{
				Pattern:  in.Pattern,
				Tags:     in.Tags,
				Verbose:  in.Verbose,
				Timeout:  in.Timeout,
				Race:     in.Race,
				Parallel: in.Parallel,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("run tests failed: %w", err)
			}

			output, _ := json.Marshal(report)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *testingPack) coverageReportTool() tool.Tool {
	return tool.NewBuilder("test_coverage_report").
		WithDescription("Generate a test coverage report").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pattern string `json:"pattern,omitempty"`
				Format  string `json:"format,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			report, err := p.cfg.Runner.GetCoverage(ctx, CoverageOptions{
				Pattern: in.Pattern,
				Format:  in.Format,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("coverage report failed: %w", err)
			}

			output, _ := json.Marshal(report)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *testingPack) benchmarkTool() tool.Tool {
	return tool.NewBuilder("test_benchmark").
		WithDescription("Run benchmark tests").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pattern string `json:"pattern,omitempty"`
				Count   int    `json:"count,omitempty"`
				Time    string `json:"time,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			report, err := p.cfg.Runner.RunBenchmark(ctx, BenchmarkOptions{
				Pattern: in.Pattern,
				Count:   in.Count,
				Time:    in.Time,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("benchmark failed: %w", err)
			}

			output, _ := json.Marshal(report)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *testingPack) generateTestTool() tool.Tool {
	return tool.NewBuilder("test_generate_test").
		WithDescription("Generate test code for source code").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Source    string `json:"source"`
				Language  string `json:"language,omitempty"`
				Style     string `json:"style,omitempty"`
				Framework string `json:"framework,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Source == "" {
				return tool.Result{}, fmt.Errorf("source is required")
			}

			generated, err := p.cfg.Generator.GenerateTest(ctx, in.Source, in.Language, GenerateOptions{
				Style:     in.Style,
				Framework: in.Framework,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("test generation failed: %w", err)
			}

			output, _ := json.Marshal(generated)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *testingPack) mutationTestTool() tool.Tool {
	return tool.NewBuilder("test_mutation_test").
		WithDescription("Run mutation testing to verify test quality").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Pattern  string   `json:"pattern,omitempty"`
				Mutators []string `json:"mutators,omitempty"`
				Timeout  string   `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			report, err := p.cfg.Mutator.MutationTest(ctx, MutationOptions{
				Pattern:  in.Pattern,
				Mutators: in.Mutators,
				Timeout:  in.Timeout,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("mutation testing failed: %w", err)
			}

			output, _ := json.Marshal(report)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
