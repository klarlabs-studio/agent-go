// Package finance provides financial calculation tools for agents.
package finance

import (
	"context"
	"encoding/json"
	"math"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the finance tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("finance").
		WithDescription("Financial calculation utilities").
		AddTools(
			compoundInterestTool(),
			loanPaymentTool(),
			loanAmortizationTool(),
			presentValueTool(),
			futureValueTool(),
			npvTool(),
			irrTool(),
			roiTool(),
			breakEvenTool(),
			depreciationTool(),
			taxTool(),
			currencyRoundTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func compoundInterestTool() tool.Tool {
	return tool.NewBuilder("finance_compound_interest").
		WithDescription("Calculate compound interest").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Principal float64 `json:"principal"`
				Rate      float64 `json:"rate"`                // Annual rate as decimal (0.05 = 5%)
				Time      float64 `json:"time"`                // Years
				Compounds int     `json:"compounds,omitempty"` // Per year (default 12)
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			n := params.Compounds
			if n <= 0 {
				n = 12 // Monthly compounding
			}

			// A = P(1 + r/n)^(nt)
			amount := params.Principal * math.Pow(1+params.Rate/float64(n), float64(n)*params.Time)
			interest := amount - params.Principal

			result := map[string]any{
				"principal":       params.Principal,
				"rate":            params.Rate,
				"rate_percent":    params.Rate * 100,
				"time_years":      params.Time,
				"compounds_year":  n,
				"final_amount":    amount,
				"interest_earned": interest,
				"total_periods":   n * int(params.Time),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func loanPaymentTool() tool.Tool {
	return tool.NewBuilder("finance_loan_payment").
		WithDescription("Calculate monthly loan payment").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Principal float64 `json:"principal"`
				Rate      float64 `json:"rate"` // Annual rate as decimal
				Years     float64 `json:"years"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			monthlyRate := params.Rate / 12
			numPayments := params.Years * 12

			// M = P * (r(1+r)^n) / ((1+r)^n - 1)
			var monthlyPayment float64
			if monthlyRate == 0 {
				monthlyPayment = params.Principal / numPayments
			} else {
				monthlyPayment = params.Principal *
					(monthlyRate * math.Pow(1+monthlyRate, numPayments)) /
					(math.Pow(1+monthlyRate, numPayments) - 1)
			}

			totalPayment := monthlyPayment * numPayments
			totalInterest := totalPayment - params.Principal

			result := map[string]any{
				"principal":       params.Principal,
				"annual_rate":     params.Rate,
				"years":           params.Years,
				"monthly_payment": monthlyPayment,
				"total_payments":  int(numPayments),
				"total_paid":      totalPayment,
				"total_interest":  totalInterest,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func loanAmortizationTool() tool.Tool {
	return tool.NewBuilder("finance_amortization").
		WithDescription("Generate loan amortization schedule").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Principal float64 `json:"principal"`
				Rate      float64 `json:"rate"` // Annual rate
				Years     float64 `json:"years"`
				MaxRows   int     `json:"max_rows,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			monthlyRate := params.Rate / 12
			numPayments := int(params.Years * 12)

			var monthlyPayment float64
			if monthlyRate == 0 {
				monthlyPayment = params.Principal / float64(numPayments)
			} else {
				monthlyPayment = params.Principal *
					(monthlyRate * math.Pow(1+monthlyRate, float64(numPayments))) /
					(math.Pow(1+monthlyRate, float64(numPayments)) - 1)
			}

			maxRows := params.MaxRows
			if maxRows <= 0 || maxRows > numPayments {
				maxRows = numPayments
			}

			type Payment struct {
				Period    int     `json:"period"`
				Payment   float64 `json:"payment"`
				Principal float64 `json:"principal"`
				Interest  float64 `json:"interest"`
				Balance   float64 `json:"balance"`
			}

			balance := params.Principal
			schedule := make([]Payment, 0, maxRows)
			totalInterest := 0.0
			totalPrincipal := 0.0

			for i := 1; i <= numPayments; i++ {
				interestPart := balance * monthlyRate
				principalPart := monthlyPayment - interestPart
				balance -= principalPart

				totalInterest += interestPart
				totalPrincipal += principalPart

				if i <= maxRows {
					schedule = append(schedule, Payment{
						Period:    i,
						Payment:   monthlyPayment,
						Principal: principalPart,
						Interest:  interestPart,
						Balance:   max(0, balance),
					})
				}
			}

			result := map[string]any{
				"monthly_payment": monthlyPayment,
				"total_payments":  numPayments,
				"total_interest":  totalInterest,
				"total_principal": totalPrincipal,
				"schedule":        schedule,
				"rows_shown":      len(schedule),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func presentValueTool() tool.Tool {
	return tool.NewBuilder("finance_present_value").
		WithDescription("Calculate present value").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				FutureValue float64 `json:"future_value"`
				Rate        float64 `json:"rate"`
				Periods     int     `json:"periods"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// PV = FV / (1 + r)^n
			presentValue := params.FutureValue / math.Pow(1+params.Rate, float64(params.Periods))
			discount := params.FutureValue - presentValue

			result := map[string]any{
				"present_value": presentValue,
				"future_value":  params.FutureValue,
				"rate":          params.Rate,
				"periods":       params.Periods,
				"discount":      discount,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func futureValueTool() tool.Tool {
	return tool.NewBuilder("finance_future_value").
		WithDescription("Calculate future value").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				PresentValue float64 `json:"present_value"`
				Rate         float64 `json:"rate"`
				Periods      int     `json:"periods"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// FV = PV * (1 + r)^n
			futureValue := params.PresentValue * math.Pow(1+params.Rate, float64(params.Periods))
			growth := futureValue - params.PresentValue

			result := map[string]any{
				"future_value":  futureValue,
				"present_value": params.PresentValue,
				"rate":          params.Rate,
				"periods":       params.Periods,
				"growth":        growth,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func npvTool() tool.Tool {
	return tool.NewBuilder("finance_npv").
		WithDescription("Calculate Net Present Value").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				CashFlows []float64 `json:"cash_flows"` // First is initial investment (negative)
				Rate      float64   `json:"rate"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			npv := 0.0
			for i, cf := range params.CashFlows {
				npv += cf / math.Pow(1+params.Rate, float64(i))
			}

			result := map[string]any{
				"npv":         npv,
				"rate":        params.Rate,
				"periods":     len(params.CashFlows),
				"is_positive": npv > 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func irrTool() tool.Tool {
	return tool.NewBuilder("finance_irr").
		WithDescription("Calculate Internal Rate of Return").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				CashFlows []float64 `json:"cash_flows"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Newton-Raphson method
			rate := 0.1 // Initial guess
			maxIterations := 100
			tolerance := 0.0001

			for i := 0; i < maxIterations; i++ {
				npv := 0.0
				npvDeriv := 0.0

				for j, cf := range params.CashFlows {
					t := float64(j)
					npv += cf / math.Pow(1+rate, t)
					if j > 0 {
						npvDeriv -= t * cf / math.Pow(1+rate, t+1)
					}
				}

				if math.Abs(npvDeriv) < 1e-10 {
					break
				}

				newRate := rate - npv/npvDeriv
				if math.Abs(newRate-rate) < tolerance {
					rate = newRate
					break
				}
				rate = newRate
			}

			result := map[string]any{
				"irr":         rate,
				"irr_percent": rate * 100,
				"periods":     len(params.CashFlows),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func roiTool() tool.Tool {
	return tool.NewBuilder("finance_roi").
		WithDescription("Calculate Return on Investment").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Investment float64 `json:"investment"`
				Returns    float64 `json:"returns"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Investment == 0 {
				result := map[string]any{"error": "investment cannot be zero"}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			profit := params.Returns - params.Investment
			roi := (profit / params.Investment) * 100

			result := map[string]any{
				"investment":  params.Investment,
				"returns":     params.Returns,
				"profit":      profit,
				"roi_percent": roi,
				"is_positive": profit > 0,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func breakEvenTool() tool.Tool {
	return tool.NewBuilder("finance_break_even").
		WithDescription("Calculate break-even point").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				FixedCosts   float64 `json:"fixed_costs"`
				PricePerUnit float64 `json:"price_per_unit"`
				CostPerUnit  float64 `json:"cost_per_unit"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			contributionMargin := params.PricePerUnit - params.CostPerUnit

			if contributionMargin <= 0 {
				result := map[string]any{
					"error":               "price per unit must be greater than cost per unit",
					"contribution_margin": contributionMargin,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			breakEvenUnits := params.FixedCosts / contributionMargin
			breakEvenRevenue := breakEvenUnits * params.PricePerUnit

			result := map[string]any{
				"break_even_units":    breakEvenUnits,
				"break_even_revenue":  breakEvenRevenue,
				"fixed_costs":         params.FixedCosts,
				"contribution_margin": contributionMargin,
				"margin_ratio":        contributionMargin / params.PricePerUnit,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func depreciationTool() tool.Tool {
	return tool.NewBuilder("finance_depreciation").
		WithDescription("Calculate asset depreciation").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Cost    float64 `json:"cost"`
				Salvage float64 `json:"salvage"`
				Life    int     `json:"life_years"`
				Method  string  `json:"method,omitempty"` // straight_line, declining_balance, sum_of_years
				Rate    float64 `json:"rate,omitempty"`   // For declining balance
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			method := params.Method
			if method == "" {
				method = "straight_line"
			}

			depreciable := params.Cost - params.Salvage

			type YearDepreciation struct {
				Year         int     `json:"year"`
				Depreciation float64 `json:"depreciation"`
				BookValue    float64 `json:"book_value"`
			}

			schedule := make([]YearDepreciation, params.Life)
			bookValue := params.Cost

			switch method {
			case "straight_line":
				annualDep := depreciable / float64(params.Life)
				for i := 0; i < params.Life; i++ {
					bookValue -= annualDep
					schedule[i] = YearDepreciation{
						Year:         i + 1,
						Depreciation: annualDep,
						BookValue:    max(bookValue, params.Salvage),
					}
				}

			case "declining_balance":
				rate := params.Rate
				if rate == 0 {
					rate = 2.0 / float64(params.Life) // Double declining
				}
				for i := 0; i < params.Life; i++ {
					dep := bookValue * rate
					if bookValue-dep < params.Salvage {
						dep = bookValue - params.Salvage
					}
					bookValue -= dep
					schedule[i] = YearDepreciation{
						Year:         i + 1,
						Depreciation: dep,
						BookValue:    max(bookValue, params.Salvage),
					}
				}

			case "sum_of_years":
				sumYears := params.Life * (params.Life + 1) / 2
				for i := 0; i < params.Life; i++ {
					remaining := params.Life - i
					dep := depreciable * float64(remaining) / float64(sumYears)
					bookValue -= dep
					schedule[i] = YearDepreciation{
						Year:         i + 1,
						Depreciation: dep,
						BookValue:    max(bookValue, params.Salvage),
					}
				}
			}

			result := map[string]any{
				"cost":               params.Cost,
				"salvage":            params.Salvage,
				"life_years":         params.Life,
				"method":             method,
				"depreciable_amount": depreciable,
				"schedule":           schedule,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func taxTool() tool.Tool {
	return tool.NewBuilder("finance_tax").
		WithDescription("Calculate tax based on brackets").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Income   float64 `json:"income"`
				Brackets []struct {
					Min  float64 `json:"min"`
					Max  float64 `json:"max"`
					Rate float64 `json:"rate"`
				} `json:"brackets"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			totalTax := 0.0
			remaining := params.Income

			type BracketTax struct {
				Min     float64 `json:"min"`
				Max     float64 `json:"max"`
				Rate    float64 `json:"rate"`
				Taxable float64 `json:"taxable"`
				Tax     float64 `json:"tax"`
			}

			breakdown := make([]BracketTax, 0)

			for _, bracket := range params.Brackets {
				if remaining <= 0 {
					break
				}

				taxableInBracket := 0.0
				if params.Income > bracket.Min {
					if bracket.Max > 0 {
						taxableInBracket = min(params.Income, bracket.Max) - bracket.Min
					} else {
						taxableInBracket = params.Income - bracket.Min
					}
					taxableInBracket = min(taxableInBracket, remaining)
				}

				if taxableInBracket > 0 {
					tax := taxableInBracket * bracket.Rate
					totalTax += tax
					remaining -= taxableInBracket

					breakdown = append(breakdown, BracketTax{
						Min:     bracket.Min,
						Max:     bracket.Max,
						Rate:    bracket.Rate,
						Taxable: taxableInBracket,
						Tax:     tax,
					})
				}
			}

			effectiveRate := 0.0
			if params.Income > 0 {
				effectiveRate = totalTax / params.Income
			}

			result := map[string]any{
				"income":         params.Income,
				"total_tax":      totalTax,
				"after_tax":      params.Income - totalTax,
				"effective_rate": effectiveRate,
				"breakdown":      breakdown,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func currencyRoundTool() tool.Tool {
	return tool.NewBuilder("finance_currency_round").
		WithDescription("Round to currency precision").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Amount   float64 `json:"amount"`
				Decimals int     `json:"decimals,omitempty"` // default 2
				Mode     string  `json:"mode,omitempty"`     // round, floor, ceil
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			decimals := params.Decimals
			if decimals == 0 {
				decimals = 2
			}

			multiplier := math.Pow(10, float64(decimals))
			value := params.Amount * multiplier

			var rounded float64
			switch params.Mode {
			case "floor":
				rounded = math.Floor(value) / multiplier
			case "ceil":
				rounded = math.Ceil(value) / multiplier
			default:
				rounded = math.Round(value) / multiplier
			}

			result := map[string]any{
				"original": params.Amount,
				"rounded":  rounded,
				"decimals": decimals,
				"mode":     params.Mode,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
