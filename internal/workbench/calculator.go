package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type CalculatorInput struct {
	Expression string `json:"expression"`
	Variables  string `json:"variables"`
	Angle      string `json:"angle"`
}

var calcName = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*$`)
var calcNumber = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`)
var calcFunctions = map[string]int{"sqrt": 1, "abs": 1, "exp": 1, "ln": 1, "log": 1, "log10": 1, "log2": 1, "sin": 1, "cos": 1, "tan": 1, "asin": 1, "acos": 1, "atan": 1, "floor": 1, "ceil": 1, "round": 1, "pow": 2, "min": 2, "max": 2, "atan2": 2}
var calcConstants = map[string]float64{"pi": math.Pi, "e": math.E, "kB": 1.380649e-23, "qe": 1.602176634e-19}
var calcScales = map[string]float64{"f": 1e-15, "p": 1e-12, "n": 1e-9, "u": 1e-6, "m": 1e-3, "k": 1e3, "K": 1e3, "M": 1e6, "meg": 1e6, "G": 1e9, "T": 1e12}

type expressionParser struct {
	text       string
	pos, depth int
	angle      string
	resolve    func(string) (float64, error)
}

func (p *expressionParser) skip() {
	for p.pos < len(p.text) && strings.ContainsRune(" \t\r\n", rune(p.text[p.pos])) {
		p.pos++
	}
}
func (p *expressionParser) take(s string) bool {
	p.skip()
	if strings.HasPrefix(p.text[p.pos:], s) {
		p.pos += len(s)
		return true
	}
	return false
}
func (p *expressionParser) expr(min int) (float64, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 64 {
		return 0, errors.New("expression nesting limit")
	}
	var v float64
	var err error
	if p.take("+") {
		v, err = p.expr(3)
	} else if p.take("-") {
		v, err = p.expr(3)
		v = -v
	} else if p.take("(") {
		v, err = p.expr(0)
		if err == nil && !p.take(")") {
			err = errors.New("missing closing parenthesis")
		}
	} else {
		p.skip()
		part := p.text[p.pos:]
		number := calcNumber.FindString(part)
		if number != "" {
			p.pos += len(number)
			v, err = strconv.ParseFloat(number, 64)
			start := p.pos
			for p.pos < len(p.text) && ((p.text[p.pos] >= 'a' && p.text[p.pos] <= 'z') || (p.text[p.pos] >= 'A' && p.text[p.pos] <= 'Z')) {
				p.pos++
			}
			if p.pos > start {
				scale, ok := calcScales[p.text[start:p.pos]]
				if !ok {
					return 0, errors.New("unknown numeric suffix; use f p n u m k M meg G T")
				}
				v *= scale
			}
		} else {
			start := p.pos
			for p.pos < len(p.text) && ((p.text[p.pos] >= 'a' && p.text[p.pos] <= 'z') || (p.text[p.pos] >= 'A' && p.text[p.pos] <= 'Z') || p.text[p.pos] == '_' || (p.pos > start && p.text[p.pos] >= '0' && p.text[p.pos] <= '9')) {
				p.pos++
			}
			name := p.text[start:p.pos]
			if name == "" {
				return 0, fmt.Errorf("expected a number, variable or function at %d", p.pos+1)
			}
			if p.take("(") {
				arity, ok := calcFunctions[name]
				if !ok {
					return 0, fmt.Errorf("unknown function %s", name)
				}
				args := []float64{}
				for i := 0; i < arity; i++ {
					if i > 0 && !p.take(",") {
						return 0, errors.New("missing argument separator")
					}
					a, e := p.expr(0)
					if e != nil {
						return 0, e
					}
					args = append(args, a)
				}
				if !p.take(")") {
					return 0, errors.New("wrong argument count or missing parenthesis")
				}
				v = calcFunction(name, args, p.angle)
			} else {
				v, err = p.resolve(name)
			}
		}
	}
	if err != nil {
		return 0, err
	}
	for {
		p.skip()
		if p.pos == len(p.text) {
			break
		}
		op := p.text[p.pos]
		prec := 0
		switch op {
		case '+', '-':
			prec = 1
		case '*', '/', '%':
			prec = 2
		case '^':
			prec = 4
		default:
			return v, nil
		}
		if prec < min {
			break
		}
		p.pos++
		next := prec + 1
		if op == '^' {
			next = prec
		}
		r, e := p.expr(next)
		if e != nil {
			return 0, e
		}
		switch op {
		case '+':
			v += r
		case '-':
			v -= r
		case '*':
			v *= r
		case '/':
			if r == 0 {
				return 0, errors.New("division by zero")
			}
			v /= r
		case '%':
			if r == 0 {
				return 0, errors.New("modulo by zero")
			}
			v = math.Mod(v, r)
		case '^':
			v = math.Pow(v, r)
		}
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("non-finite result or function domain error")
	}
	return v, nil
}
func calcFunction(n string, a []float64, angle string) float64 {
	x := a[0]
	factor := 1.0
	if angle == "deg" {
		factor = math.Pi / 180
	}
	switch n {
	case "sqrt":
		return math.Sqrt(x)
	case "abs":
		return math.Abs(x)
	case "exp":
		return math.Exp(x)
	case "ln", "log":
		return math.Log(x)
	case "log10":
		return math.Log10(x)
	case "log2":
		return math.Log2(x)
	case "sin":
		return math.Sin(x * factor)
	case "cos":
		return math.Cos(x * factor)
	case "tan":
		return math.Tan(x * factor)
	case "asin":
		return math.Asin(x) / factor
	case "acos":
		return math.Acos(x) / factor
	case "atan":
		return math.Atan(x) / factor
	case "floor":
		return math.Floor(x)
	case "ceil":
		return math.Ceil(x)
	case "round":
		return math.Round(x)
	case "pow":
		return math.Pow(x, a[1])
	case "min":
		return math.Min(x, a[1])
	case "max":
		return math.Max(x, a[1])
	case "atan2":
		return math.Atan2(x, a[1]) / factor
	}
	return math.NaN()
}
func calculate(raw json.RawMessage) (any, error) {
	var x CalculatorInput
	if e := parseInput(raw, &x); e != nil {
		return nil, e
	}
	if len(x.Expression) == 0 || len(x.Expression) > 4096 || len(x.Variables) > 16000 {
		return nil, errors.New("expression required; expression/variables limit 4096/16000 bytes")
	}
	if x.Angle != "" && x.Angle != "rad" && x.Angle != "deg" {
		return nil, errors.New("angle must be rad or deg")
	}
	definitions := map[string]string{}
	values := map[string]float64{}
	visiting := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(x.Variables, ";", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("variables use name = expression, one per line")
		}
		name := strings.TrimSpace(parts[0])
		_, reserved := calcConstants[name]
		if !calcName.MatchString(name) || reserved || calcFunctions[name] > 0 || definitions[name] != "" {
			return nil, fmt.Errorf("invalid, duplicate or reserved variable %s", name)
		}
		definitions[name] = strings.TrimSpace(parts[1])
		if definitions[name] == "" {
			return nil, errors.New("variable expression missing")
		}
	}
	if len(definitions) > 64 {
		return nil, errors.New("max 64 variables")
	}
	var resolve func(string) (float64, error)
	eval := func(text string) (float64, error) {
		p := expressionParser{text: text, angle: x.Angle, resolve: resolve}
		v, e := p.expr(0)
		p.skip()
		if e == nil && p.pos != len(text) {
			e = fmt.Errorf("unexpected input at %d", p.pos+1)
		}
		return v, e
	}
	resolve = func(name string) (float64, error) {
		if v, ok := values[name]; ok {
			return v, nil
		}
		if v, ok := calcConstants[name]; ok {
			return v, nil
		}
		expr, ok := definitions[name]
		if !ok {
			return 0, fmt.Errorf("undefined variable %s", name)
		}
		if visiting[name] {
			return 0, fmt.Errorf("circular variable %s", name)
		}
		visiting[name] = true
		v, e := eval(expr)
		visiting[name] = false
		if e == nil {
			values[name] = v
		}
		return v, e
	}
	for name := range definitions {
		if _, e := resolve(name); e != nil {
			return nil, e
		}
	}
	v, e := eval(x.Expression)
	if e != nil {
		return nil, e
	}
	return map[string]any{"value": v, "formatted": strconv.FormatFloat(v, 'g', 12, 64), "variables": values, "expression": x.Expression, "angle": x.Angle}, nil
}
