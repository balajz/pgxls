package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/balajz/pgxls/ast"
	"github.com/balajz/pgxls/ast/astutil"
	"github.com/balajz/pgxls/dialect"
	"github.com/balajz/pgxls/parser"
	"github.com/balajz/pgxls/parser/parseutil"
	"github.com/balajz/pgxls/pkg/database"
	"github.com/balajz/pgxls/pkg/types"
	"github.com/balajz/pgxls/token"
)

type completionType int

const (
	_ completionType = iota
	CompletionTypeKeyword
	CompletionTypeFunction
	CompletionTypeColumn
	CompletionTypeTable
	CompletionTypeReferencedTable
	CompletionTypeView
	CompletionTypeSubQuery
	CompletionTypeSubQueryColumn
	CompletionTypeChange
	CompletionTypeUser
	CompletionTypeSchema
	CompletionTypeJoin
	CompletionTypeJoinOn
)

func (ct completionType) String() string {
	switch ct {
	case CompletionTypeKeyword:
		return "Keyword"
	case CompletionTypeFunction:
		return "Function"
	case CompletionTypeColumn:
		return "Column"
	case CompletionTypeTable:
		return "Table"
	case CompletionTypeReferencedTable:
		return "ReferencedTable"
	case CompletionTypeView:
		return "View"
	case CompletionTypeChange:
		return "Change"
	case CompletionTypeUser:
		return "User"
	case CompletionTypeSchema:
		return "Schema"
	case CompletionTypeSubQuery:
		return "SubQuery"
	case CompletionTypeSubQueryColumn:
		return "SubQueryColumn"
	case CompletionTypeJoin:
		return "Join clause"
	case CompletionTypeJoinOn:
		return "Join On condition"
	default:
		return ""
	}
}

type Completer struct {
	DBCache *database.DBCache
	Driver  dialect.DatabaseDriver
}

func NewCompleter(dbCache *database.DBCache) *Completer {
	return &Completer{
		DBCache: dbCache,
	}
}

func completionTypeIs(completionTypes []completionType, expect completionType) bool {
	return slices.Contains(completionTypes, expect)
}

func (c *Completer) Complete(text string, line, col int, lowercaseKeywords bool) ([]types.CompletionItem, error) {
	parsed, err := parser.Parse(text)
	if err != nil {
		return nil, err
	}

	pos := token.Pos{
		Line: line,
		Col:  col,
	}

	nodeWalker := parseutil.NewNodeWalker(parsed, pos)
	ctx := getCompletionTypes(nodeWalker)

	definedTables, err := parseutil.ExtractTable(parsed, pos)
	if err != nil {
		return nil, err
	}
	definedSubQueries, err := parseutil.ExtractSubQueryViews(parsed, pos)
	if err != nil {
		return nil, err
	}

	lastWord := getLastWord(text, line+1, col)
	withBackQuote := strings.HasPrefix(lastWord, "`")

	if ctx.syntaxPos == parseutil.TableReference && strings.HasPrefix(strings.ToUpper(lastWord), "WH") {
		ctx.types = []completionType{
			CompletionTypeColumn,
			CompletionTypeTable,
			CompletionTypeReferencedTable,
			CompletionTypeView,
			CompletionTypeSubQueryColumn,
			CompletionTypeSubQuery,
			CompletionTypeFunction,
			CompletionTypeKeyword,
		}
	}

	var items []types.CompletionItem

	if c.DBCache != nil {
		if completionTypeIs(ctx.types, CompletionTypeColumn) {
			candidates := c.columnCandidates(definedTables, ctx.parent)
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(items, candidates...)
		}
		if completionTypeIs(ctx.types, CompletionTypeReferencedTable) {
			candidates := c.ReferencedTableCandidates(definedTables)
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(items, candidates...)
		}
		if completionTypeIs(ctx.types, CompletionTypeTable) {
			excl := definedTables
			if completionTypeIs(ctx.types, CompletionTypeJoin) {
				excl = nil
			}
			candidates := c.TableCandidates(ctx.parent, excl)
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(items, candidates...)
		}
		if completionTypeIs(ctx.types, CompletionTypeSchema) {
			candidates := c.SchemaCandidates()
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(items, candidates...)
		}
		if completionTypeIs(ctx.types, CompletionTypeSubQuery) {
			candidates := c.SubQueryCandidates(definedSubQueries)
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(items, candidates...)
		}
		if completionTypeIs(ctx.types, CompletionTypeSubQueryColumn) {
			candidates := c.SubQueryColumnCandidates(definedSubQueries)
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(items, candidates...)
		}
		joinOn := completionTypeIs(ctx.types, CompletionTypeJoinOn)
		if completionTypeIs(ctx.types, CompletionTypeJoin) || joinOn {
			table, err := parseutil.ExtractLastTable(parsed, pos)
			if err != nil {
				return nil, err
			}
			tables, err := parseutil.ExtractPrevTables(parsed, pos)
			if err != nil {
				return nil, err
			}
			candidates := c.joinCandidates(table, tables, definedTables, joinOn, lowercaseKeywords)
			if withBackQuote {
				candidates = toQuotedCandidates(candidates, c.Driver)
			}
			items = append(candidates, items...)
		}
	}

	if completionTypeIs(ctx.types, CompletionTypeKeyword) {
		drivers := dialect.DataBaseKeywords(c.Driver)
		items = append(items, c.keywordCandidates(lowercaseKeywords, drivers)...)
	}
	if completionTypeIs(ctx.types, CompletionTypeFunction) {
		drivers := dialect.DataBaseFunctions(c.Driver)
		items = append(items, c.functionCandidates(lowercaseKeywords, drivers)...)
	}

	items = filterCandidates(items, lastWord)
	populateSortText(items)

	return items, nil
}

// Override the sort text for each completion item.
func populateSortText(items []types.CompletionItem) {
	for i := range items {
		items[i].SortText = getSortTextPrefix(items[i].Kind) + items[i].Label
	}
}

// Some completion kinds are more relevant than others.
// This prefix defines the alphabetic priority of each kind.
func getSortTextPrefix(kind types.CompletionItemKind) string {
	switch kind {
	case types.SnippetCompletion:
		return "00"
	case types.FieldCompletion:
		return "0"
	case types.ClassCompletion:
		return "1"
	case types.ModuleCompletion:
		return "2"
	case types.FunctionCompletion:
		return "10"
	case
		types.ColorCompletion,
		types.ConstantCompletion,
		types.ConstructorCompletion,
		types.EnumCompletion,
		types.EnumMemberCompletion,
		types.EventCompletion,
		types.FileCompletion,
		types.FolderCompletion,
		types.InterfaceCompletion,
		types.KeywordCompletion,
		types.MethodCompletion,
		types.OperatorCompletion,
		types.PropertyCompletion,
		types.ReferenceCompletion,
		types.StructCompletion,
		types.TextCompletion,
		types.TypeParameterCompletion,
		types.UnitCompletion,
		types.ValueCompletion,
		types.VariableCompletion:
		return "9999"
	default:
		return "9999"
	}
}

type ParentType int

const (
	_ ParentType = iota
	ParentTypeNone
	ParentTypeSchema
	ParentTypeTable
	ParentTypeSubQuery
)

type completionParent struct {
	Type ParentType
	Name string
}

var noneParent = &completionParent{Type: ParentTypeNone}

type CompletionContext struct {
	types     []completionType
	parent    *completionParent
	syntaxPos parseutil.SyntaxPosition
}

func getCompletionTypes(nw *parseutil.NodeWalker) *CompletionContext {
	memberIdentifierMatcher := astutil.NodeMatcher{
		NodeTypes: []ast.NodeType{ast.TypeMemberIdentifier},
	}

	syntaxPos := parseutil.CheckSyntaxPosition(nw)
	var t []completionType
	p := noneParent
	switch syntaxPos {

	case parseutil.ColName:
		if nw.CurNodeIs(memberIdentifierMatcher) {
			// has parent
			mi := nw.CurNodeTopMatched(memberIdentifierMatcher).(*ast.MemberIdentifier)
			t = []completionType{
				CompletionTypeColumn,
				CompletionTypeSubQueryColumn,
				CompletionTypeView,
			}
			p = &completionParent{
				Type: ParentTypeTable,
				Name: mi.Parent.String(),
			}
		} else {
			t = []completionType{
				CompletionTypeColumn,
				CompletionTypeTable,
				CompletionTypeReferencedTable,
				CompletionTypeSubQueryColumn,
				CompletionTypeSubQuery,
				CompletionTypeView,
				CompletionTypeFunction,
			}
			p = noneParent
		}
	case parseutil.AliasName:
		// pass
	case parseutil.SelectExpr, parseutil.CaseValue:
		if nw.CurNodeIs(memberIdentifierMatcher) {
			// has parent
			mi := nw.CurNodeTopMatched(memberIdentifierMatcher).(*ast.MemberIdentifier)
			t = []completionType{
				CompletionTypeColumn,
				CompletionTypeView,
				CompletionTypeSubQueryColumn,
			}
			p = &completionParent{
				Type: ParentTypeTable,
				Name: mi.ParentTok.NoQuoteString(),
			}
		} else {
			t = []completionType{
				CompletionTypeColumn,
				CompletionTypeTable,
				CompletionTypeReferencedTable,
				CompletionTypeView,
				CompletionTypeSubQueryColumn,
				CompletionTypeSubQuery,
				CompletionTypeFunction,
				CompletionTypeKeyword,
			}
		}
	case parseutil.TableReference:
		if nw.CurNodeIs(memberIdentifierMatcher) {
			// has parent
			mi := nw.CurNodeTopMatched(memberIdentifierMatcher).(*ast.MemberIdentifier)
			t = []completionType{
				CompletionTypeTable,
				CompletionTypeView,
				CompletionTypeSubQueryColumn,
			}
			p = &completionParent{
				Type: ParentTypeSchema,
				Name: mi.ParentTok.NoQuoteString(),
			}
		} else {
			t = []completionType{
				CompletionTypeTable,
				CompletionTypeReferencedTable,
				CompletionTypeSchema,
				CompletionTypeView,
				CompletionTypeSubQuery,
				CompletionTypeKeyword,
			}
		}
	case parseutil.WhereCondition:
		if nw.CurNodeIs(memberIdentifierMatcher) {
			// has parent
			mi := nw.CurNodeTopMatched(memberIdentifierMatcher).(*ast.MemberIdentifier)
			t = []completionType{
				CompletionTypeColumn,
				CompletionTypeView,
				CompletionTypeSubQueryColumn,
			}
			p = &completionParent{
				Type: ParentTypeTable,
				Name: mi.ParentTok.NoQuoteString(),
			}
		} else {
			t = []completionType{
				CompletionTypeColumn,
				CompletionTypeTable,
				CompletionTypeReferencedTable,
				CompletionTypeView,
				CompletionTypeSubQueryColumn,
				CompletionTypeSubQuery,
				CompletionTypeFunction,
				CompletionTypeKeyword,
			}
		}
	case parseutil.JoinClause:
		t = []completionType{
			CompletionTypeJoin,
			CompletionTypeTable,
			CompletionTypeReferencedTable,
			CompletionTypeSchema,
			CompletionTypeView,
			CompletionTypeSubQuery,
			CompletionTypeKeyword,
		}
	case parseutil.JoinOn:
		t = []completionType{
			CompletionTypeJoinOn,
			CompletionTypeColumn,
			CompletionTypeReferencedTable,
			CompletionTypeSubQueryColumn,
			CompletionTypeSubQuery,
		}
	case parseutil.InsertColumn:
		t = []completionType{
			CompletionTypeColumn,
			CompletionTypeView,
		}
	default:
		t = []completionType{
			CompletionTypeKeyword,
		}
	}
	return &CompletionContext{
		types:     t,
		parent:    p,
		syntaxPos: syntaxPos,
	}
}

func filterCandidates(candidates []types.CompletionItem, lastWord string) []types.CompletionItem {
	filtered := []types.CompletionItem{}
	upperLastWord := strings.ToUpper(lastWord)
	for _, candidate := range candidates {
		upperLabel := strings.ToUpper(candidate.Label)
		if strings.HasPrefix(upperLabel, upperLastWord) && upperLabel != upperLastWord {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func getLine(text string, line int) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	i := 1
	for scanner.Scan() {
		if i == line {
			return scanner.Text()
		}
		i++
	}
	return ""
}

func getLastWord(text string, line, char int) string {
	t := getBeforeCursorText(text, line, char)
	s := getLine(t, line)

	reg := regexp.MustCompile("[\\w`]+$")
	ss := reg.FindAllString(s, -1)
	if len(ss) == 0 {
		return ""
	}
	return ss[len(ss)-1]
}

func getBeforeCursorText(text string, line, char int) string {
	writer := bytes.NewBufferString("")
	scanner := bufio.NewScanner(strings.NewReader(text))

	i := 1
	for scanner.Scan() {
		if i == line {
			t := scanner.Text()
			writer.Write([]byte(t[:char]))
			break
		}
		fmt.Fprintln(writer, scanner.Text())
		i++
	}
	return writer.String()
}

func toQuotedCandidates(candidates []types.CompletionItem, _ dialect.DatabaseDriver) []types.CompletionItem {
	quotedCandidates := make([]types.CompletionItem, len(candidates))
	for i, candidate := range candidates {
		candidate.Label = fmt.Sprintf("\"%s\"", candidate.Label)
		quotedCandidates[i] = candidate
	}
	return quotedCandidates
}
