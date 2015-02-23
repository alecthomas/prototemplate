package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"unicode"
	"unicode/utf8"

	"code.google.com/p/goprotobuf/proto"
	"github.com/alecthomas/kingpin"
	"github.com/alecthomas/template"
	"github.com/robertkrimen/otto"

	"github.com/alecthomas/prototemplate/gen/google/protobuf"
)

var (
	TemplateDir = ""

	listFlag     = kingpin.Flag("list", "List builtin generators.").Dispatch(listGenerators).Bool()
	builtinsFlag = kingpin.Flag("builtins", "List builtin functions.").Dispatch(listBuiltins).Bool()

	includesFlag    = kingpin.Flag("include", "List of include paths to pass to protoc.").Short('I').PlaceHolder("DIR").Strings()
	templateDirFlag = kingpin.Flag("templates", "Root path to templates.").Default(TemplateDir).ExistingDir()
	outputFlag      = kingpin.Flag("output", "File to output generated template source to.").Short('o').PlaceHolder("FILE").String()

	sourceArg   = kingpin.Arg("proto", "Protocol buffer definition to compile.").Required().ExistingFile()
	templateArg = kingpin.Arg("template", "Template file, or name of a builtin generator.").Required().String()
	scriptArg   = kingpin.Arg("script", "A JavaScript file defining template helper functions.").ExistingFile()
)

const builtins = `
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
`

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
	vm := otto.New()
	_, err := vm.Run(builtins)
	if err != nil {
		return err
	}
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

// Regenerate protobuf source with:
// protoc --go_out=./gen -I/usr/local/Cellar/protobuf/2.6.0/include \
//   /usr/local/Cellar/protobuf/2.6.0/include/google/protobuf/descriptor.proto
//

type TemplateContext struct {
	FileDescriptorSet *google_protobuf.FileDescriptorSet
	Types             map[string]int32
	Labels            map[string]int32
}

func main() {
	kingpin.Parse()

	bareWord := filepath.Base(*templateArg) == *templateArg && filepath.Ext(*templateArg) == ""
	if bareWord {
		name := *templateArg
		*templateArg = filepath.Join(*templateDirFlag, name, name+".got")
		*scriptArg = filepath.Join(*templateDirFlag, name, name+".js")
	}

	tmpl, err := template.New(filepath.Base(*templateArg)).Funcs(buildFunctions()).ParseFiles(*templateArg)
	kingpin.FatalIfError(err, "")

	cmd := exec.Command("protoc", protoArgs()...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		kingpin.Fatalf("%s", output)
	}

	pb := &google_protobuf.FileDescriptorSet{}
	err = proto.Unmarshal(output, pb)
	kingpin.FatalIfError(err, "")

	var w io.WriteCloser = os.Stdout
	if *outputFlag != "" {
		w, err = os.Create(*outputFlag)
		kingpin.FatalIfError(err, "")
	}

	// bytes, _ := json.MarshalIndent(pb, "", "  ")
	// fmt.Printf("%s\n", bytes)
	context := &TemplateContext{
		FileDescriptorSet: pb,
		Types:             google_protobuf.FieldDescriptorProto_Type_value,
		Labels:            google_protobuf.FieldDescriptorProto_Label_value,
	}
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

type Func func(args ...interface{}) (interface{}, error)

func buildFunctions() template.FuncMap {
	funcs := template.FuncMap{}
	if *scriptArg == "" {
		return funcs
	}
	vm := otto.New()
	_, err := vm.Run(builtins)
	kingpin.FatalIfError(err, "")
	injectProtoSymbols(vm)
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
		labels.Set(name, value)
	}
	vm.Set("Labels", labels)

	types, _ := vm.Object("({})")
	for name, value := range google_protobuf.FieldDescriptorProto_Type_value {
		types.Set(name, value)
	}
	vm.Set("Types", types)
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
