package main

// Regenerate protobuf source with:
//   make gen

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/otto"
	"github.com/alecthomas/template"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	TemplateDir = ""

	includesFlag         = kingpin.Flag("include", "List of include paths to pass to protoc.").Short('I').PlaceHolder("DIR").Strings()
	templateDirFlag      = kingpin.Flag("templates", "Root path to templates.").Default(TemplateDir).ExistingDir()
	printTemplateDirFlag = kingpin.Flag("print-template-dir", "Print default template directory.").PreAction(printTemplateDir).Bool()
	outputFlag           = kingpin.Flag("output", "File to output generated template source to.").Short('o').PlaceHolder("FILE").String()
	setFlag              = kingpin.Flag("set", "Pass a variable to the JS and template context.").PlaceHolder("K=V").StringMap()
	printContextFlag     = kingpin.Flag("print-context", "Print template context.").Bool()

	sourceArg   = kingpin.Arg("proto", "Protocol buffer definition to compile.").Required().ExistingFile()
	templateArg = kingpin.Arg("template", "Template file, or name of a builtin generator.").Required().String()
	scriptArg   = kingpin.Arg("script", "A JavaScript file defining template helper functions.").ExistingFile()
)

func init() {
	kingpin.Flag("list-generators", "List builtin generators.").PreAction(listGenerators).Bool()
	kingpin.Flag("list-functions", "List builtin functions.").PreAction(listBuiltins).Bool()
}

const builtins = `
var tagTypeMap = {}
tagTypeMap[Types.TYPE_DOUBLE] = 1;
tagTypeMap[Types.TYPE_FLOAT] = 5;
tagTypeMap[Types.TYPE_INT64] = 0;
tagTypeMap[Types.TYPE_UINT64] = 0;
tagTypeMap[Types.TYPE_INT32] = 0;
tagTypeMap[Types.TYPE_FIXED64] = 1;
tagTypeMap[Types.TYPE_FIXED32] = 5;
tagTypeMap[Types.TYPE_BOOL] = 0;
tagTypeMap[Types.TYPE_STRING] = 2;
tagTypeMap[Types.TYPE_MESSAGE] = 2;
tagTypeMap[Types.TYPE_BYTES] = 2;
tagTypeMap[Types.TYPE_UINT32] = 0;
tagTypeMap[Types.TYPE_ENUM] = 0;
tagTypeMap[Types.TYPE_SFIXED32] = 5;
tagTypeMap[Types.TYPE_SFIXED64] = 1;
tagTypeMap[Types.TYPE_SINT32] = 0;
tagTypeMap[Types.TYPE_SINT64] = 0;

function IsOptional(t) {
  return t.Label == Labels.LABEL_OPTIONAL;
}

function IsRequired(f) {
  return f.Label == Labels.LABEL_REQUIRED;
}

function IsEnum(f) {
  return f.Type == Types.TYPE_ENUM;
}

function IsRepeated(f) {
  return f.Label == Labels.LABEL_REPEATED;
}

function IsMessage(f) {
  return f.Type == Types.TYPE_MESSAGE;
}

function FieldTag(f) {
  return (f.Number << 3) | tagTypeMap[f.Type];
}

function FixRef(ref) {
  return Type(ref.replace(/^.*\./, ''));
}

function UpperCamelCase(str) {
  return str.split(/_/g).map(function (txt) {return txt.charAt(0).toUpperCase() + txt.substr(1);}).join("")
}

function LowerCamelCase(str) {
  var name = UpperCamelCase(str);
  return name.charAt(0).toLowerCase() + name.substr(1);
}

function UpperSnakeCase(str) {
  return str.split(/_/g).map(function (txt) {return txt.charAt(0).toUpperCase() + txt.substr(1).toLowerCase();}).join("")
}

function StripModule(ref) {
  return ref.replace(/^.*\./, '');
}

function FieldsOrOneOf(fields, oneof) {
  var out = []
  for (var i in fields) {
    if (fields[i].OneofIndex === undefined)
      out.push(fields[i]);
  }
  for (var i in oneof) {
    var v = oneof[i];
    v.OneofIndex = i;
    out.push(v);
  }
  return out;
}

function FieldsWithoutOneOf(fields) {
  var out = [];
  for (var i in fields) {
    if (fields[i].OneofIndex === undefined)
      out.push(fields[i]);
  }
  return out;
}

function OneOfFields(index, fields) {
  var out = []
  for (var i in fields) {
    if (fields[i].OneofIndex === index)
      out.push(fields[i]);
  }
  return out;
}

function IsOneOfField(index, field) {
  return field.OneofIndex !== undefined && index == field.OneofIndex;
}

`

func printTemplateDir(*kingpin.ParseContext) error {
	fmt.Println(TemplateDir)
	os.Exit(0)
	return nil
}

func listGenerators(*kingpin.ParseContext) error {
	files, err := ioutil.ReadDir(TemplateDir)
	if err != nil {
		return fmt.Errorf("invalid template dir '%s': %s", TemplateDir, err)
	}
	for _, file := range files {
		if file.IsDir() {
			fmt.Println(file.Name())
		}
	}
	os.Exit(0)
	return nil
}

func listBuiltins(*kingpin.ParseContext) error {
	vm := buildVM(nil)
	helpers, _ := vm.Run(`Function('return this')();`)
	object := helpers.Object()
	for _, name := range object.Keys() {
		if isExported(name) {
			fmt.Println(name)
		}
	}
	os.Exit(0)
	return nil
}

func isExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func main() {
	kingpin.Parse()

	if *printTemplateDirFlag {
		fmt.Println(TemplateDir)
		return
	}

	bareWord := filepath.Base(*templateArg) == *templateArg && filepath.Ext(*templateArg) == ""
	if bareWord {
		name := *templateArg
		*templateArg = filepath.Join(*templateDirFlag, name, name+".got")
		*scriptArg = filepath.Join(*templateDirFlag, name, name+".js")
	}

	cmd := exec.Command("protoc", protoArgs()...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		kingpin.Fatalf("%s", output)
	}

	pb := &google_protobuf.FileDescriptorSet{}
	err = proto.Unmarshal(output, pb)
	kingpin.FatalIfError(err, "")

	tmpl, err := template.New(filepath.Base(*templateArg)).Funcs(buildFunctions(pb)).ParseFiles(*templateArg)
	kingpin.FatalIfError(err, "")

	var w io.WriteCloser = os.Stdout
	if *outputFlag != "" {
		w, err = os.Create(*outputFlag)
		kingpin.FatalIfError(err, "")
	}

	if *printContextFlag {
		bytes, _ := json.MarshalIndent(pb, "", "  ")
		fmt.Printf("%s\n", bytes)
		return
	}

	context := parseUserVars()
	context["FileDescriptorSet"] = pb
	context["Types"] = google_protobuf.FieldDescriptorProto_Type_value
	context["Labels"] = google_protobuf.FieldDescriptorProto_Label_value
	err = tmpl.Execute(w, context)
	kingpin.FatalIfError(err, "")
}

func protoArgs() []string {
	args := []string{}
	for _, include := range *includesFlag {
		args = append(args, "-I"+include)
	}
	args = append(args, []string{
		"--include_source_info",
		"--descriptor_set_out=/dev/stdout",
		*sourceArg,
	}...)
	return args
}

func parseUserVars() map[string]interface{} {
	out := map[string]interface{}{}
	for k, jv := range *setFlag {
		var v interface{}
		if err := json.Unmarshal([]byte(jv), &v); err != nil {
			v = jv
		}
		out[k] = v
	}
	return out
}

func toValue(vm *otto.Otto, v interface{}) otto.Value {
	value, err := vm.ToValue(v)
	kingpin.FatalIfError(err, "")
	return value
}

func buildVM(pb *google_protobuf.FileDescriptorSet) *otto.Otto {
	vm := otto.New()

	err := vm.Set("FindMessage", func(call otto.FunctionCall) otto.Value {
		name := call.Argument(0).String()
		for _, file := range pb.File {
			for _, typ := range file.MessageType {
				if typ.GetName() == name {
					return toValue(vm, typ)
				}
			}
		}
		return otto.Value{}
	})

	err = vm.Set("FindEnum", func(call otto.FunctionCall) otto.Value {
		name := call.Argument(0).String()
		for _, file := range pb.File {
			for _, typ := range file.EnumType {
				if typ.GetName() == name {
					return toValue(vm, typ)
				}
			}
		}
		return otto.Value{}
	})

	kingpin.FatalIfError(err, "")

	injectProtoSymbols(vm)

	_, err = vm.Run(builtins)
	kingpin.FatalIfError(err, "")
	for k, v := range parseUserVars() {
		err = vm.Set(k, v)
		kingpin.FatalIfError(err, "")
	}
	return vm
}

type Func func(args ...interface{}) (interface{}, error)

func buildFunctions(pb *google_protobuf.FileDescriptorSet) template.FuncMap {
	funcs := template.FuncMap{}
	if *scriptArg == "" {
		return funcs
	}
	vm := buildVM(pb)
	source, err := ioutil.ReadFile(*scriptArg)
	kingpin.FatalIfError(err, "")
	script, err := vm.Compile(*scriptArg, source)
	kingpin.FatalIfError(err, "")
	_, err = vm.Run(script)
	kingpin.FatalIfError(err, "")
	helpers, err := vm.Run(`Function('return this')();`)
	if !helpers.IsObject() {
		kingpin.Fatalf("expected top-level object with helper functions in %s", *scriptArg)
	}
	object := helpers.Object()
	for _, name := range object.Keys() {
		if !isExported(name) {
			continue
		}
		f, _ := object.Get(name)
		if !f.IsFunction() {
			continue
		}
		funcs[name] = func(name string) Func {
			return func(args ...interface{}) (interface{}, error) {
				fargs := []interface{}{}
				for _, arg := range args {
					v, err := vm.ToValue(toGenericValue(arg))
					kingpin.FatalIfError(err, "")
					fargs = append(fargs, v)
				}
				value, err := object.Call(name, fargs...)
				if err != nil {
					return nil, err
				}
				return value.Export()
			}
		}(name)
	}
	return funcs
}

func injectProtoSymbols(vm *otto.Otto) {
	labels, _ := vm.Object("({})")
	for name, value := range google_protobuf.FieldDescriptorProto_Label_value {
		_ = labels.Set(name, value)
	}
	_ = vm.Set("Labels", labels)

	types, _ := vm.Object("({})")
	for name, value := range google_protobuf.FieldDescriptorProto_Type_value {
		_ = types.Set(name, value)
	}
	_ = vm.Set("Types", types)
}

// Otto does not fully support mapping Go types directly to otto.Values. This
// works around that issue by recursively converting a Go structure to nested
// basic types (map, slice, float, etc.).
func toGenericValue(v interface{}) interface{} {
	rv := reflect.Indirect(reflect.ValueOf(v))
	switch rv.Kind() {
	case reflect.Struct:
		out := map[string]interface{}{}
		for i := 0; i < rv.NumField(); i++ {
			t := rv.Type().Field(i)
			if t.PkgPath != "" {
				continue
			}
			fv := rv.Field(i)
			out[t.Name] = toGenericValue(fv.Interface())
		}
		return out

	case reflect.Slice, reflect.Array:
		out := []interface{}{}
		for i := 0; i < rv.Len(); i++ {
			out = append(out, toGenericValue(rv.Index(i).Interface()))
		}
		return out

	case reflect.Map:
		out := map[interface{}]interface{}{}
		for _, key := range rv.MapKeys() {
			value := rv.MapIndex(key)
			out[toGenericValue(key.Interface())] = toGenericValue(value.Interface())
		}
		return out
	}
	return v
}
