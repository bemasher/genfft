/*
   genfft - A code parser/generator to generate Go from annotated lists created with the FFTW tool genfft.
   Copyright (C) 2014  Douglas Hall

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published
   by the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bufio"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// An expression consists of either an Operator or a Literal. Operators have
// at least one operand which are Expressions.
type Expression struct {
	Op    string
	Lit   string
	OpIdx int
	Expr  []*Expression
}

// Determines if an operator is unary based on the number of operands.
func (expr Expression) IsUnary() bool {
	return len(expr.Expr) == 1
}

// Produces valid Go string representation of the expression.
func (expr Expression) String() string {
	return expr.stringHelper(0)
}

// String() provides default depth value, this is the actual function.
func (expr Expression) stringHelper(depth int) (r string) {
	// If the expression is an Operand, just print the literal.
	if expr.Op == "" {
		return expr.Lit
	}

	// Only print parenthesis if we're not at the top level.
	if depth > 1 {
		r += "("
		defer func() {
			r += ")"
		}()
	}

	// Only use the combined declaration//assignment operator if we're assigning to a T\d+ variable.
	if expr.Op == "=" && strings.HasPrefix(expr.Expr[0].Lit, "T") {
		expr.Op = ":="
	}

	// If the expression is unary then print the operator before it's operand.
	if expr.IsUnary() {
		r += expr.Op
		r += expr.Expr[0].stringHelper(depth + 1)
	} else {
		// For each operand.
		for idx := range expr.Expr {
			// Don't print an operator on the first operand.
			if idx != 0 {
				// If the operator is addition and the current operand is unary subtraction, simplify.
				if expr.Op == "+" && expr.Expr[idx].IsUnary() && expr.Expr[idx].Op == "-" {
					r += "-"
					r += expr.Expr[idx].Expr[0].stringHelper(depth + 1)
					continue
				} else {
					// Else, append the operator.
					r += expr.Op
				}
			}
			// Recurse for each operand. This is skipped if we did unary simplification.
			r += expr.Expr[idx].stringHelper(depth + 1)
		}
	}

	return
}

// Parse an expression into an expression tree.
func (expr *Expression) Parse(line string) {
	// Skip '(:'
	expr.parse(line[2:])
}

// Parse(line string) skips first two characters of each expression, this is
// the actual function.
func (expr *Expression) parse(line string) (idx int) {
	// Operators always have at least one operand.
	// Empty expressions will never appear in String() output but we may want
	// to conditionally create this later for large parse trees.
	expr.Expr = append(expr.Expr, new(Expression))

	// While there are characters left to parse.
	for idx = 0; idx < len(line); idx++ {
		switch line[idx] {
		// Opening parenthesis represent the beginning of a new expression.
		// Recursively parse it and update index with number of characters
		// consumed.
		case '(':
			idx += expr.Expr[expr.OpIdx].parse(line[idx+1:])
		// Colons are the first character in the assignment operator, we don't need it so skip it.
		case ':':
			continue
		// Operators are appended to Op field and space following is consumed.
		case '=', '+', '-', '*':
			expr.Op = line[idx : idx+1]
			idx += 1
		// Spaces will only ever be encountered between operands, so make a
		// new operand and increment the operand index.
		case ' ':
			expr.Expr = append(expr.Expr, new(Expression))
			expr.OpIdx++
		// Closing parenthesis represent termination of an expression. Return
		// consumed character count + 1 to include this character.
		case ')':
			return idx + 1
		// Anything else must be part of a literal, append it.
		default:
			expr.Expr[expr.OpIdx].Lit += line[idx : idx+1]
		}
	}

	return
}

func (expr *Expression) TransformLength() uint {
	return expr.transformLength() + 1
}

func (expr *Expression) transformLength() (max uint) {
	if strings.HasSuffix(expr.Lit, "]") {
		n, err := strconv.ParseUint(expr.Lit[3:len(expr.Lit)-1], 10, 64)
		if err != nil {
			panic(err)
		}
		return uint(n)
	}

	for _, child := range expr.Expr {
		max = Max(max, child.transformLength())
	}

	return
}

func Max(a ...uint) (max uint) {
	for _, i := range a {
		if i > max {
			max = i
		}
	}
	return max
}

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	// Print usage if user didn't pass the right number of arguments.
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s [filename]\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	// Open input file for reading.
	inputFile, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal("Error opening input file", err)
	}
	defer inputFile.Close()

	// Create a line scanner, we only need input one line at a time.
	inputScanner := bufio.NewScanner(inputFile)

	var (
		expressions []Expression
		constants   [][]string
	)

	// Regular expression matches constants.
	constRe := regexp.MustCompile(`(DV?K)\((K[NP][\d_]+), ([+-][\d\.]+)\);`)

	// For each line in the input.
	for idx := 0; inputScanner.Scan(); idx++ {
		// Get the current line.
		line := inputScanner.Text()

		// If the regex matches we have a constant.
		if constRe.MatchString(line) {
			// Append the matching groups from the constant to the constant list.
			submatches := constRe.FindStringSubmatch(line)
			constants = append(constants, submatches[2:])

			// Continue so we don't try parsing this as an expression.
			continue
		}

		// Parse the line and append it to the body.
		var expr Expression
		expr.Parse(line)

		expressions = append(expressions, expr)
	}

	// Lay out some boilerplate for the go formatter.
	source := "package dft\n"

	var max uint
	for _, expr := range expressions {
		max = Max(max, expr.TransformLength())
	}
	source += fmt.Sprintf("const N = %d\n\n", max)

	// Append the constants we parsed including the imaginary constant.
	source += "const (\n"
	source += "I = 1i\n"
	for _, constant := range constants {
		source += fmt.Sprintf("%s = %s\n", constant[0], constant[1])
	}
	source += ")\n"

	// Append the function signature, body and enclosing brackets.
	// Note: I'm probably going to regret hardcoding this comparison.
	if expressions[0].Expr[1].Lit == "xi[0]" {
		source += "func DFT(xi, xo []complex128) {\n"
	} else {
		source += "func DFT(ri, ii, ro, io []float64) {\n"
	}

	for _, expr := range expressions {
		source += expr.String() + "\n"
	}

	source += "}"

	// Format the source.
	formatted, err := format.Source([]byte(source))
	if err != nil {
		log.Fatal("Error formatting go source:", err)
	}

	// Display the formatted source.
	os.Stdout.Write(formatted)
}
