package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer/stateful"
	"github.com/dave/jennifer/jen"
	log "github.com/sirupsen/logrus"
)

// Program is a list of constants and expressions.
type Program struct {
	Constants  []Constant
	Statements []Expr `@@+`
}

// Gen creates a go-representation of the program.
func (p Program) Gen(path, name string) *jen.File {
	// Make a set of function arguments.
	argSet := map[string]struct{}{}
	// For each statement.
	for _, s := range p.Statements {
		// Add inputs to the argument set.
		for _, input := range s.Inputs() {
			argSet[input] = struct{}{}
		}
	}

	var (
		args    []jen.Code
		argType jen.Code
	)
	if _, exists := argSet["ri"]; exists {
		// If the argSet contains "ri", it's a float dft.
		args = []jen.Code{
			jen.Id("ri"), jen.Id("ii"),
			jen.Id("ro"), jen.Id("io"),
		}
		argType = jen.Float64()
	} else {
		// Otherwise it's a complex dft.
		args = []jen.Code{jen.Id("xi"), jen.Id("xo")}
		argType = jen.Complex128()
		// Always include the imaginary constant first.
		p.Constants = append([]Constant{{"I", "1i"}}, p.Constants...)
	}

	f := jen.NewFilePathName(path, path)

	// Define a named function.
	f.Func().Id(name).Params(
		// Add arguments, and their type ([]float64, []complex128).
		jen.List(args...).Index().Add(argType),
	).BlockFunc(func(g *jen.Group) {
		// If there are any constants.
		if len(p.Constants) > 0 {
			// Render them.
			g.Add(jen.Const().DefsFunc(func(d *jen.Group) {
				for _, c := range p.Constants {
					d.Add(c.Gen())
				}
			}))

			// Add a blank line.
			g.Line()
		}

		// Render the statements.
		for _, expr := range p.Statements {
			g.Add(expr.Gen())
		}
	})

	return f
}

// Constant is a named value.
type Constant struct {
	Name  string
	Value string
}

func (c Constant) Gen() (s *jen.Statement) {
	return jen.Id(c.Name).Op("=").Id(c.Value)
}

// Expr is an Ident or an Op and at least one sub-expression.
type Expr struct {
	Ident string `@Id |`
	Op    string `"(" @Op`
	Sub   []Expr `@@+ ")"`
}

func (e Expr) String() string {
	if e.Ident != "" {
		return fmt.Sprintf("%q", e.Ident)
	}

	return fmt.Sprintf("{Op:%q Sub:%+v}", e.Op, e.Sub)
}

// Inputs walks an expression tree and returns a list of identifiers with indices.
func (e Expr) Inputs() (i []string) {
	idx := strings.IndexByte(e.Ident, '[')

	if idx != -1 {
		i = append(i, e.Ident[:idx])
	}

	for _, sub := range e.Sub {
		i = append(i, sub.Inputs()...)
	}

	return
}

// Gen renders a go-representation of an expression.
func (e Expr) Gen() (c *jen.Statement) {
	// Expressions with an identifier are just that identifier.
	if e.Ident != "" {
		return jen.Id(e.Ident)
	}

	// Expressions with only one sub-expression render the operator and that sub-expression.
	if len(e.Sub) == 1 {
		return jen.Op(e.Op).Add(e.Sub[0].Gen())
	}

	// When the left side of an expression is an indexed identifier, assign only.
	if e.Op == ":=" && strings.HasSuffix(e.Sub[0].Ident, "]") {
		e.Op = "="
	}

	// Render the left side of the expression.
	lt := e.Sub[0].Gen()
	for _, sub := range e.Sub[1:] {
		// If right side of multiply is a binary operation, wrap in parentheses.
		if e.Op == "*" && len(sub.Sub) > 1 {
			lt.Add(jen.Op(e.Op)).Parens(sub.Gen())
			continue
		}

		// Flatten add followed by unary subtraction.
		if e.Op == "+" && sub.Op == "-" {
			lt.Add(sub.Gen())
			continue
		}
		lt.Add(jen.Op(e.Op)).Add(sub.Gen())
	}

	return lt
}

type Dft struct {
	Prefix string `json:"prefix"`
	Func   string `json:"func"`
}

func init() {
	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Dir(f) + "\\"

	log.SetFormatter(&log.TextFormatter{
		ForceColors: true,
		CallerPrettyfier: func(frame *runtime.Frame) (fn, file string) {
			file = strings.TrimPrefix(filepath.Clean(frame.File), dir)
			return frame.Function, fmt.Sprintf("%s:%d", file, frame.Line)
		},
	})
	log.SetReportCaller(true)
	log.SetLevel(log.TraceLevel)
}

func main() {
	// Token rules for schedule files.
	def := stateful.MustSimple([]stateful.Rule{
		{Name: "Lt", Pattern: `\(`},
		{Name: "Rt", Pattern: `\)`},
		{Name: "Id", Pattern: `[a-zA-Z][a-zA-Z0-9_]*(\[\d+\])?`},
		{Name: "Op", Pattern: `(:=|[+\-*])`},
		{Name: "eol", Pattern: `[\r\n]+`},
		{Name: "sp", Pattern: `\s+`},
	})

	// Constant regular expression.
	constRe := regexp.MustCompile(`^\s*DV?K\((.*?), (.*?)\);$`)

	// Build a parser from main.Program
	parser := participle.MustBuild(&Program{}, participle.Lexer(def))

	// Load configurations.
	dfts := []Dft{}

	configBytes, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("%+v\n", fmt.Errorf("os.ReadFile: %w", err))
	}

	err = json.Unmarshal(configBytes, &dfts)
	if err != nil {
		log.Fatalf("%+v\n", fmt.Errorf("json.Unmarshal: %w", err))
	}

	for _, dft := range dfts {
		// Function wrapper provides scope for defer statements.
		func() {
			alstFilename := dft.Prefix + ".alst"
			coutFilename := dft.Prefix + ".cout"
			goFilename := dft.Prefix + ".go"

			// Open the schedule file.
			alstFile, err := os.Open(alstFilename)
			if err != nil {
				log.Fatalf("%+v\n", fmt.Errorf("os.Open: %w", err))
			}
			defer alstFile.Close()

			prog := &Program{}

			// Parse the schedule.
			err = parser.Parse(alstFilename, alstFile, prog)
			if err != nil {
				log.Fatalf("%+v\n", fmt.Errorf("parser.Parse: %w", err))
			}

			// Open the C output.
			coutFile, err := os.Open(coutFilename)
			if err != nil {
				log.Fatalf("%+v\n", fmt.Errorf("os.Open: %w", err))
			}
			defer coutFile.Close()

			// Create a new line scanner.
			coutScanner := bufio.NewScanner(coutFile)

			// Scan lines from coutFile.
			for coutScanner.Scan() {
				line := coutScanner.Text()

				// If the line isn't a constant, bail.
				if !constRe.MatchString(line) {
					continue
				}

				// Parse the constant and append it to the program.
				m := constRe.FindStringSubmatch(line)
				prog.Constants = append(
					prog.Constants,
					Constant{Name: m[1], Value: m[2]},
				)
			}

			// Generate code from the program.
			f := prog.Gen("dft", dft.Func)

			// Write the code to disk.
			log.Infof("writing %s\n", goFilename)
			err = f.Save(goFilename)
			if err != nil {
				log.Fatalf("%+v\n", fmt.Errorf("f.Save: %w", err))
			}
		}()
	}
}
