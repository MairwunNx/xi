package commands

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrNoMatch       = errors.New("no matching pattern found")
	ErrEmptyInput    = errors.New("input is empty")
	ErrInvalidSchema = errors.New("invalid schema format")
)

type Schema struct {
	Name   string
	Parts  []SchemaPart
	source string
}

type SchemaPart struct {
	IsArg bool
	Name  string
}

type ParseResult struct {
	Schema string
	Args   map[string]string
}

func (r *ParseResult) Get(name string) string {
	return r.Args[name]
}

func (r *ParseResult) Has(name string) bool {
	_, ok := r.Args[name]
	return ok
}

type Parser struct {
	schemas []Schema
}

func NewParser() *Parser {
	return &Parser{
		schemas: make([]Schema, 0),
	}
}

func (p *Parser) Register(pattern string) error {
	schema, err := parseSchema(pattern)
	if err != nil {
		return fmt.Errorf("register pattern %q: %w", pattern, err)
	}
	p.schemas = append(p.schemas, schema)
	return nil
}

func (p *Parser) MustRegister(patterns ...string) *Parser {
	for _, pattern := range patterns {
		if err := p.Register(pattern); err != nil {
			panic(err)
		}
	}
	return p
}

func (p *Parser) Parse(input string) (*ParseResult, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrEmptyInput
	}

	tokens := tokenize(input)
	if len(tokens) == 0 {
		return nil, ErrEmptyInput
	}

	for _, schema := range p.schemas {
		if result, ok := matchSchema(schema, tokens); ok {
			return result, nil
		}
	}

	return nil, ErrNoMatch
}

var argPattern = regexp.MustCompile(`^\{([a-zA-Z_][a-zA-Z0-9_]*)\}$`)

func parseSchema(pattern string) (Schema, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return Schema{}, ErrInvalidSchema
	}

	parts := strings.Fields(pattern)
	schemaParts := make([]SchemaPart, 0, len(parts))
	var name strings.Builder

	for i, part := range parts {
		if matches := argPattern.FindStringSubmatch(part); matches != nil {
			schemaParts = append(schemaParts, SchemaPart{
				IsArg: true,
				Name:  matches[1],
			})
			if i > 0 {
				name.WriteString(" ")
			}
			name.WriteString(fmt.Sprintf("{%s}", matches[1]))
		} else {
			schemaParts = append(schemaParts, SchemaPart{
				IsArg: false,
				Name:  part,
			})
			if i > 0 {
				name.WriteString(" ")
			}
			name.WriteString(part)
		}
	}

	return Schema{
		Name:   name.String(),
		Parts:  schemaParts,
		source: pattern,
	}, nil
}

func tokenize(input string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if escaped {
			if ch == '\'' || ch == '\\' {
				current.WriteByte(ch)
			} else {
				current.WriteByte('\\')
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == '\'' {
			inQuotes = !inQuotes
			continue
		}

		if ch == ' ' && !inQuotes {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func matchSchema(schema Schema, tokens []string) (*ParseResult, bool) {
	if len(tokens) != len(schema.Parts) {
		return nil, false
	}

	args := make(map[string]string)

	for i, part := range schema.Parts {
		if part.IsArg {
			args[part.Name] = tokens[i]
		} else {
			if !strings.EqualFold(tokens[i], part.Name) {
				return nil, false
			}
		}
	}

	return &ParseResult{
		Schema: schema.Name,
		Args:   args,
	}, true
}