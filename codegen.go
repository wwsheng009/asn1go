package asn1go

import (
	"errors"
	"fmt"
	goast "go/ast"
	goprint "go/printer"
	gotoken "go/token"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type CodeGenerator interface {
	Generate(module ModuleDefinition, writer io.Writer) error
}

type GenParams struct {
	Package string
	Prefix  string
	Type    GenType
}

type GenType int

const (
	GEN_DECLARATIONS GenType = iota
)

func NewCodeGenerator(params GenParams) CodeGenerator {
	switch params.Type {
	case GEN_DECLARATIONS:
		return &declCodeGen{params}
	default:
		return nil
	}
}

type declCodeGen struct {
	Params GenParams
}

type moduleContext struct {
	extensibilityImplied bool
	tagDefault           int
	errors               []error
	lookupContext        ModuleBody
	requiredModules      []string
}

func (ctx *moduleContext) appendError(err error) {
	ctx.errors = append(ctx.errors, err)
}

func (ctx *moduleContext) requireModule(module string) {
	for _, existing := range ctx.requiredModules {
		if existing == module {
			return
		}
	}
	ctx.requiredModules = append(ctx.requiredModules, module)
}

/** Generate declarations from module

Feature support status:
 - [x] ModuleIdentifier
 - [ ] TagDefault
 - [ ] ExtensibilityImplied
 - [.] ModuleBody -- see generateDeclarations
*/
func (gen declCodeGen) Generate(module ModuleDefinition, writer io.Writer) error {
	ctx := moduleContext{
		extensibilityImplied: module.ExtensibilityImplied,
		tagDefault:           module.TagDefault,
		lookupContext:        module.ModuleBody,
	}
	moduleName := goast.NewIdent(goifyName(module.ModuleIdentifier.Reference))
	if len(gen.Params.Package) > 0 {
		moduleName = goast.NewIdent(gen.Params.Package)
	}
	ast := &goast.File{
		Name:  moduleName,
		Decls: ctx.generateDeclarations(module),
	}
	if len(ctx.errors) != 0 {
		msg := "Errors generating Go AST from module: \n"
		for _, err := range ctx.errors {
			msg += "  " + err.Error() + "\n"
		}
		return errors.New(msg)
	}
	for _, v := range module.ModuleBody.Imports {

		mname := goifyName(v.Module.Reference)
		ctx.requireModule(mname)
		// for _, Symbol := range v.SymbolList {
		// 	switch t := Symbol.(type) {
		// 	case TypeReference:
		// 		{
		// 			name1 = mname + "/" + goifyName(t.Name())
		// 			ctx.requireModule(name1)
		// 		}
		// 	case ModuleReference:
		// 		{
		// 			name1 = mname + "/" + goifyName(t.Name())
		// 			ctx.requireModule(name1)
		// 		}
		// 	case ValueReference:
		// 		{
		// 			name1 = mname + "/" + goifyName(t.Name())
		// 			ctx.requireModule(name1)
		// 		}
		// 	}
		// }

	}

	importDecls := make([]goast.Decl, 0)
	for _, moduleName := range ctx.requiredModules {
		modulePath := &goast.BasicLit{Kind: gotoken.STRING, Value: fmt.Sprintf("\"%v\"", moduleName)}
		specs := []goast.Spec{&goast.ImportSpec{Path: modulePath}}
		importDecls = append(importDecls, &goast.GenDecl{Tok: gotoken.IMPORT, Specs: specs})
	}

	ast.Decls = append(importDecls, ast.Decls...)
	return goprint.Fprint(writer, gotoken.NewFileSet(), ast)
}
func IsUpper(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func IsLower(s string) bool {
	for _, r := range s {
		if !unicode.IsLower(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
func goifyName(name string) string {
	if name == "BigInt" {
		return "int64"
	} else if name == "StringStore" {
		return "string"
	}
	caser := cases.Title(language.English, cases.NoLower)
	//when all letter is upper
	tmp := strings.ReplaceAll(name, "-", "")
	if IsUpper(tmp) {
		return caser.String(strings.Replace(name, "-", "_", -1))
	}

	name1 := strings.ReplaceAll(name, "-", "_")
	names := strings.Split(name1, "_")

	for i := 0; i < len(names); i++ {
		names[i] = caser.String(names[i])
	}
	return strings.Join(names, "")
	// return strings.Title(strings.Replace(name, "-", "_", -1))
}

/** generateDeclarations based on ModuleBody of module

Feature support status:
 - [.] AssignmentList
   - [ ] ValueAssignment
   - [x] TypeAssignment
 - [ ] Imports
*/
func (ctx *moduleContext) generateDeclarations(module ModuleDefinition) []goast.Decl {
	decls := make([]goast.Decl, 0)
	for _, assignment := range module.ModuleBody.AssignmentList {
		switch a := assignment.(type) {
		case TypeAssignment:
			decls = append(decls, ctx.generateTypeDecl(a.TypeReference, a.Type))
		case ValueAssignment:
			// decls = append(decls, ctx.generateValueCommentDecl(a.ValueReference, a.Type, a.Value))
			decls = append(decls, ctx.generateValueDecl(a.ValueReference, a.Type, a.Value))
			// fmt.Println("not support yet")
		}
	}

	return decls
}

// func (ctx *moduleContext) generateValueCommentDecl(reference ValueReference, typeDescr Type, value Value) goast.Decl {
// 	return &goast.GenDecl{
// 		Tok: gotoken.COMMENT,
// 		Specs: []goast.Spec{
// 			&goast.ValueSpec{
// 				Comment: ctx.commentFromType(typeDescr, reference.Name()),
// 			},
// 		},
// 	}
// }
func (ctx *moduleContext) generateValueDecl(reference ValueReference, typeDescr Type, value Value) goast.Decl {

	names, values := ctx.generateValueBody(value)
	return &goast.GenDecl{
		Tok: gotoken.CONST,
		Specs: []goast.Spec{
			&goast.ValueSpec{
				Names:   names,
				Type:    ctx.generateTypeBody(typeDescr, true),
				Comment: ctx.commentFromType(typeDescr, reference.Name()),
				Values:  values,
			},
		},
	}
}
func (ctx *moduleContext) generateTypeDecl(reference TypeReference, typeDescr Type) goast.Decl {
	return &goast.GenDecl{
		Tok: gotoken.TYPE,
		Specs: []goast.Spec{
			&goast.TypeSpec{
				Name:    goast.NewIdent(goifyName(reference.Name())),
				Type:    ctx.generateTypeBody(typeDescr, true),
				Comment: ctx.commentFromType(typeDescr, reference.Name()),
			},
		},
	}
}
func (ctx *moduleContext) generateValueBody(value Value) ([]*goast.Ident, []goast.Expr) {
	names := make([]*goast.Ident, 0)
	exprs := make([]goast.Expr, 0)
	switch tt := value.(type) {
	case ObjectIdentifierValue:
		{
			for _, v := range tt {
				switch vl := v.(type) {
				case ObjectIdElement:
					{
						names = append(names, goast.NewIdent(goifyName(vl.Name)))
						exprs = append(exprs, goast.NewIdent(fmt.Sprint(vl.Id)))
					}
				}
			}
		}
	}
	if len(exprs) > 0 {
		return names, exprs
	}
	return nil, nil
}
func (ctx *moduleContext) generateTypeBody(typeDescr Type, noStar Boolean) goast.Expr {
	switch t := typeDescr.(type) {
	case BooleanType:
		return goast.NewIdent("bool")
	case IntegerType:
		return goast.NewIdent("int64") // TODO signed, unsigned, range constraints
	case CharacterStringType:
		return goast.NewIdent("string")
	case RealType:
		return goast.NewIdent("float64")
	case OctetStringType:
		return &goast.ArrayType{Elt: goast.NewIdent("byte")}
	case ChoiceType:
		fields := &goast.FieldList{}
		for _, f := range t.AlternativeTypeList {
			fields.List = append(fields.List, ctx.generateStructField(NamedComponentType{NamedType: f}))
		}
		return &goast.StructType{
			Fields: fields,
		}
	case SequenceType:
		fields := &goast.FieldList{}
		for _, field := range t.Components {
			switch f := field.(type) {
			case NamedComponentType:
				fields.List = append(fields.List, ctx.generateStructField(f))
			case ComponentsOfComponentType: // TODO
			}
		}
		return &goast.StructType{
			Fields: fields,
		}
	case SetType:
		fields := &goast.FieldList{}
		for _, field := range t.Components {
			switch f := field.(type) {
			case NamedComponentType:
				fields.List = append(fields.List, ctx.generateStructField(f))
			case ComponentsOfComponentType: // TODO
			}
		}
		return &goast.StructType{
			Fields: fields,
		}
	case SetOfType:
		return &goast.ArrayType{Elt: ctx.generateTypeBody(t.Type, true)}
	case SequenceOfType:
		return &goast.ArrayType{Elt: ctx.generateTypeBody(t.Type, true)}
	case TaggedType: // TODO should put tags in go code?
		return ctx.generateTypeBody(t.Type, false)
	case ConstraintedType: // TODO should generate checking code?
		return ctx.generateTypeBody(t.Type, false)
	case TypeReference: // TODO should useful types be separate type by itself?

		prefix := "*"
		if noStar {
			prefix = ""
		}
		nameAndType := ctx.resolveTypeReference(t)
		if nameAndType != nil {
			specialCase := ctx.generateSpecialCase(*nameAndType)
			if specialCase != nil {
				return specialCase
			}
			if nameAndType.Module != "" {
				return goast.NewIdent(prefix + goifyName(nameAndType.Module) + "." + goifyName(t.Name()))
			}
		}

		return goast.NewIdent(prefix + goifyName(t.Name()))
	case RestrictedStringType: // TODO should generate checking code?
		return goast.NewIdent("string")
	case BitStringType:
		ctx.requireModule("encoding/asn1")
		return goast.NewIdent("asn1.BitString")
	case EnumeratedType:
		return goast.NewIdent("string") //在json文件中是int,虽然是int类型，但是在xml文件里是文本
	case IntegerEnumType:
		return goast.NewIdent("int") //在json文件中是int,虽然是int类型，但是在xml文件里是文本
	case StringType:
		return goast.NewIdent("string")
	case BigInt:
		return goast.NewIdent("int64")
	case NullType:
		return goast.NewIdent("interface{}")
	case ObjectIdentifierType:
		return goast.NewIdent("int")
	default:
		// NullType
		// ObjectIdentifierType
		// ChoiceType
		// RestrictedStringType
		ctx.appendError(errors.New(fmt.Sprintf("Ignoring unsupported type %#v", typeDescr)))
		return nil
	}
}

func (ctx *moduleContext) generateStructField(f NamedComponentType) *goast.Field {
	return &goast.Field{
		Names:   append(make([]*goast.Ident, 0), goast.NewIdent(goifyName(f.NamedType.Identifier.Name()))),
		Type:    ctx.generateTypeBody(f.NamedType.Type, false),
		Tag:     ctx.asn1TagFromType(f),
		Comment: ctx.commentFromComponentType(f),
	}
}
func (ctx *moduleContext) commentFromComponentType(nt NamedComponentType) *goast.CommentGroup {
	t := nt.NamedType.Type
	return ctx.commentFromType(t, nt.NamedType.Identifier.Name())
}

func (ctx *moduleContext) commentFromType(t1 Type, typeName string) *goast.CommentGroup {

	switch tt := t1.(type) {
	case ObjectIdentifierType:
		{
			return &goast.CommentGroup{List: append(make([]*goast.Comment, 0), &goast.Comment{Slash: 0, Text: fmt.Sprintf("/*%s,OID*/", goifyName(typeName))})}
		}
	case NullType:
		{
			return &goast.CommentGroup{List: append(make([]*goast.Comment, 0), &goast.Comment{Slash: 0, Text: fmt.Sprintf("//%s,NullType\n", goifyName(typeName))})}
		}
	case ChoiceType:
		{
			return &goast.CommentGroup{List: append(make([]*goast.Comment, 0), &goast.Comment{Slash: 0, Text: fmt.Sprintf("//%s,ChoiceOption\n", goifyName(typeName))})}
		}
	case IntegerEnumType:
		{
			comments := make([]string, 0)
			for _, enum := range tt.Enums {
				comments = append(comments, fmt.Sprintf("%s(%d)", enum.Name, enum.Index))
			}
			commentline := strings.Join(comments, ",")
			return &goast.CommentGroup{List: append(make([]*goast.Comment, 0), &goast.Comment{Slash: 0, Text: fmt.Sprintf("//%s,IntegerEnum:%s\n", goifyName(typeName), commentline)})}
		}
	case EnumeratedType:
		{
			comments := make([]string, 0)
			for _, enum := range tt.Enums {
				comments = append(comments, fmt.Sprintf("%s(%d)", enum.Name, enum.Index))
			}
			commentline := strings.Join(comments, ",")
			return &goast.CommentGroup{List: append(make([]*goast.Comment, 0), &goast.Comment{Slash: 0, Text: fmt.Sprintf("//%s,EnumList:%s\n", goifyName(typeName), commentline)})}
		}
	}
	return nil
}
func (ctx *moduleContext) asn1TagFromType(nt NamedComponentType) *goast.BasicLit {
	t := nt.NamedType.Type
	components := make([]string, 0)
	if nt.IsOptional {
		components = append(components, "optional")
	}
	if nt.Default != nil {
		if defaultNumber, ok := (nt.Default).(Number); ok {
			components = append(components, fmt.Sprintf("default:%v", defaultNumber.IntValue()))
		} else if defaultString, ok := (nt.Default).(String); ok {
			components = append(components, fmt.Sprintf("default:%s", defaultString.StringValue()))
		}
	}
	// unwrap type
unwrap:
	for {
		switch tt := t.(type) {
		case TaggedType:
			if tt.Tag.Class == CLASS_APPLICATION {
				components = append(components, "application")
			}
			if tt.TagType == TAGS_EXPLICIT {
				components = append(components, "explicit")
			}
			switch cn := ctx.lookupValue(tt.Tag.ClassNumber).(type) {
			case Number:
				components = append(components, fmt.Sprintf("tag:%v", cn.IntValue()))
			default:
				ctx.appendError(errors.New(fmt.Sprintf("Tag value should be Number, got %#v", cn)))
			}
			t = tt.Type
		case ConstraintedType:
			t = tt.Type
		default:
			break unwrap
		}
	}
	isReference := false
	isArray := false
	// add type-specific tags
	switch tt := t.(type) {
	case OctetStringType, SetOfType, SequenceOfType:
		isArray = true
	case RestrictedStringType:
		switch tt.LexType {
		case IA5String:
			components = append(components, "ia5")
		case UTF8String:
			components = append(components, "utf8")
		case PrintableString:
			components = append(components, "printable")
		}
	case TypeReference:
		isReference = true
		switch ctx.unwrapToLeafType(tt).TypeReference.Name() {
		case GeneralizedTimeName:
			components = append(components, "generalized")
		case UTCTimeName:
			components = append(components, "utc")
		}
		// TODO set          causes a SET, rather than a SEQUENCE type to be expected
		// TODO omitempty    causes empty slices to be skipped
	}
	//json格式把-转换成_
	id := nt.NamedType.Identifier.Name()
	json_field := strings.ReplaceAll(id, "-", "_")
	xmltag := ""
	if nt.IsOptional || isReference || isArray {
		xmltag = fmt.Sprintf("xml:\"%s\" json:\"%s,omitempty\"", id, json_field)
	} else {
		xmltag = fmt.Sprintf("xml:\"%s\" json:\"%s\"", id, json_field)
	}
	if len(components) > 0 {
		return &goast.BasicLit{
			Value: fmt.Sprintf("`%s asn1:\"%s\"`", xmltag, strings.Join(components, ",")),
			Kind:  gotoken.STRING,
		}
	} else {
		return &goast.BasicLit{
			Value: fmt.Sprintf("`%s`", xmltag),
			Kind:  gotoken.STRING,
		}
		// return nil
	}
}

func (ctx *moduleContext) generateSpecialCase(resolved TypeAssignment) goast.Expr {
	if resolved.TypeReference.Name() == GeneralizedTimeName || resolved.TypeReference.Name() == UTCTimeName {
		// time types in encoding/asn1go don't support wrapping of time.Time
		ctx.requireModule("time")
		return goast.NewIdent("time.Time")
	} else if _, ok := ctx.removeWrapperTypes(resolved.Type).(BitStringType); ok {
		ctx.requireModule("encoding/asn1")
		return goast.NewIdent("asn1.BitString")
	}
	return nil
}

// TODO really lookup values from module and imports
func (ctx *moduleContext) lookupValue(val Value) Value {
	return val
}

// resolveTypeReference resolves references until reaches unresolved type, useful type, or declared type
// returns type reference of most nested type which is not type reference itself
// returns nil if type is not resolved
func (ctx *moduleContext) resolveTypeReference(reference TypeReference) *TypeAssignment {
	unwrapped := ctx.unwrapToLeafType(reference)
	if unwrapped.Type != nil {
		return &unwrapped
	} else if tt := ctx.lookupUsefulType(unwrapped.TypeReference); tt != nil {
		module := ctx.lookupUsefulTypeModule(unwrapped.TypeReference)
		return &TypeAssignment{unwrapped.TypeReference, tt, module}
	} else {
		ctx.appendError(errors.New(fmt.Sprintf("Can not resolve TypeReference %v", reference.Name())))
		return nil
	}
}

func (ctx *moduleContext) lookupUsefulType(reference TypeReference) Type {
	if usefulType, ok := USEFUL_TYPES[reference.Name()]; ok {
		return usefulType
	} else {
		return nil
	}
}
func (ctx *moduleContext) lookupUsefulTypeModule(reference TypeReference) string {
	if usefulTypeModule, ok := USEFUL_TYPES_MODULE[reference.Name()]; ok {
		return usefulTypeModule
	} else {
		return ""
	}
}
func (ctx *moduleContext) removeWrapperTypes(t Type) Type {
	for {
		switch tt := t.(type) {
		case TaggedType:
			t = tt.Type
		case ConstraintedType:
			t = tt.Type
		default:
			return t
		}
	}
}

// unwrapToLeafType walks over transitive type references, tags and constraints and yields "root" type reference
func (ctx *moduleContext) unwrapToLeafType(reference TypeReference) TypeAssignment {
	if assignment := ctx.lookupContext.AssignmentList.GetType(reference.Name()); assignment != nil {
		t := assignment.Type
		if tt, ok := ctx.removeWrapperTypes(t).(TypeReference); ok {
			return ctx.unwrapToLeafType(tt)
		} else {
			return *assignment
		}
	}
	return TypeAssignment{reference, nil, ""}
}
