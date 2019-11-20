package gval

import (
	"bytes"
	"fmt"
	"strings"
	"text/scanner"
	"unicode"
)

type parserLang struct {
	lang            Language
	prevWhitespace  uint64
	prevMode        uint
	prevIsIdentRune func(ch rune, i int) bool
}

type parserLangStack []parserLang

func (pls *parserLangStack) push(pl parserLang) {
	*pls = append(*pls, pl)
}

func (pls parserLangStack) peek() (parserLang, bool) {
	if len(pls) == 0 {
		return parserLang{}, false
	}

	return pls[len(pls)-1], true
}

func (pls *parserLangStack) pop() (parserLang, bool) {
	pl, ok := pls.peek()
	if !ok {
		return parserLang{}, false
	}

	*pls = (*pls)[:len(*pls)-1]
	return pl, true
}

//Parser parses expressions in a Language into an Evaluable
type Parser struct {
	scanner    scanner.Scanner
	langs      parserLangStack
	lastScan   rune
	camouflage error
}

func newParser(expression string) *Parser {
	sc := scanner.Scanner{}
	sc.Init(strings.NewReader(expression))
	sc.Error = func(*scanner.Scanner, string) { return }
	sc.Filename = expression + "\t"
	return &Parser{scanner: sc}
}

func (p *Parser) currentLanguage() Language {
	pl, ok := p.langs.peek()
	if !ok {
		return Language{}
	}

	return pl.lang
}

func (p *Parser) pushLanguage(l Language) {
	if p.isCamouflaged() {
		panic("can not pushLanguage() on camouflaged Parser")
	}

	pl := parserLang{
		lang:            l,
		prevWhitespace:  p.scanner.Whitespace,
		prevMode:        p.scanner.Mode,
		prevIsIdentRune: p.scanner.IsIdentRune,
	}
	p.langs.push(pl)

	p.scanner.Whitespace = scanner.GoWhitespace
	p.scanner.Mode = scanner.GoTokens
	p.scanner.IsIdentRune = func(r rune, pos int) bool { return unicode.IsLetter(r) || r == '_' || (pos > 0 && unicode.IsDigit(r)) }
}

func (p *Parser) popLanguage() error {
	pl, ok := p.langs.pop()
	if !ok {
		return fmt.Errorf("no language to pop")
	}

	p.scanner.Whitespace = pl.prevWhitespace
	p.scanner.Mode = pl.prevMode
	p.scanner.IsIdentRune = pl.prevIsIdentRune

	return nil
}

// SetWhitespace sets the behavior of the whitespace matcher. The given
// characters must be less than or equal to 0x20 (' ').
func (p *Parser) SetWhitespace(chars ...rune) {
	var mask uint64
	for _, char := range chars {
		mask |= 1 << char
	}

	p.scanner.Whitespace = mask
}

// SetMode sets the tokens that the underlying scanner will match.
func (p *Parser) SetMode(mode uint) {
	p.scanner.Mode = mode
}

// SetIsIdentRuneFunc sets the function that matches ident characters in the
// underlying scanner.
func (p *Parser) SetIsIdentRuneFunc(fn func(ch rune, i int) bool) {
	p.scanner.IsIdentRune = fn
}

// Scan reads the next token or Unicode character from source and returns it.
// It only recognizes tokens t for which the respective Mode bit (1<<-t) is set.
// It returns scanner.EOF at the end of the source.
func (p *Parser) Scan() rune {
	if p.isCamouflaged() {
		p.camouflage = nil
		return p.lastScan
	}
	p.camouflage = nil
	p.lastScan = p.scanner.Scan()
	return p.lastScan
}

func (p *Parser) isCamouflaged() bool {
	return p.camouflage != nil && p.camouflage != errCamouflageAfterNext
}

// Camouflage rewind the last Scan(). The Parser holds the camouflage error until
// the next Scan()
// Do not call Rewind() on a camouflaged Parser
func (p *Parser) Camouflage(unit string, expected ...rune) {
	if p.isCamouflaged() {
		panic(fmt.Errorf("can only Camouflage() after Scan(): %v", p.camouflage))
	}
	p.camouflage = p.Expected(unit, expected...)
	return
}

// Peek returns the next Unicode character in the source without advancing
// the scanner. It returns EOF if the scanner's position is at the last
// character of the source.
// Do not call Peek() on a camouflaged Parser
func (p *Parser) Peek() rune {
	if p.isCamouflaged() {
		panic("can not Peek() on camouflaged Parser")
	}
	return p.scanner.Peek()
}

var errCamouflageAfterNext = fmt.Errorf("Camouflage() after Next()")

// Next reads and returns the next Unicode character.
// It returns EOF at the end of the source.
// Do not call Next() on a camouflaged Parser
func (p *Parser) Next() rune {
	if p.isCamouflaged() {
		panic("can not Next() on camouflaged Parser")
	}
	p.camouflage = errCamouflageAfterNext
	return p.scanner.Next()
}

// TokenText returns the string corresponding to the most recently scanned token.
// Valid after calling Scan().
func (p *Parser) TokenText() string {
	return p.scanner.TokenText()
}

//Expected returns an error signaling an unexpected Scan() result
func (p *Parser) Expected(unit string, expected ...rune) error {
	return unexpectedRune{unit, expected, p.lastScan}
}

type unexpectedRune struct {
	unit     string
	expected []rune
	got      rune
}

func (err unexpectedRune) Error() string {
	exp := bytes.Buffer{}
	runes := err.expected
	switch len(runes) {
	default:
		for _, r := range runes[:len(runes)-2] {
			exp.WriteString(scanner.TokenString(r))
			exp.WriteString(", ")
		}
		fallthrough
	case 2:
		exp.WriteString(scanner.TokenString(runes[len(runes)-2]))
		exp.WriteString(" or ")
		fallthrough
	case 1:
		exp.WriteString(scanner.TokenString(runes[len(runes)-1]))
	case 0:
		return fmt.Sprintf("unexpected %s while scanning %s", scanner.TokenString(err.got), err.unit)
	}
	return fmt.Sprintf("unexpected %s while scanning %s expected %s", scanner.TokenString(err.got), err.unit, exp.String())
}
