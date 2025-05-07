package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/xml"
)

func validateSVG(r io.Reader) error {

    lexer := xml.NewLexer(parse.NewInput(r))

    for {
        tt, data := lexer.Next()
        switch tt {
        case xml.ErrorToken:
            if lexer.Err() != io.EOF{
                return lexer.Err()
            }
            return nil
        case xml.StartTagToken:
           tagName := string(data)
           if strings.EqualFold(tagName, "script") {
             return fmt.Errorf("SVG contains forbidden script tags.")
           }
        case xml.AttributeToken:
          attr  := string(data)
          if strings.HasPrefix(strings.ToLower(attr), "on") {
                return fmt.Errorf("SVG contains forbidden event handler: %s", attr)
          }
        case xml.TextToken:
            if bytes.Contains(data, []byte("javascript:")) {
               return fmt.Errorf("SVG contains Javascript code.")
          }
        }
    }
}
