var typeMap = {};
typeMap[Types.TYPE_DOUBLE] = "Double";
typeMap[Types.TYPE_FLOAT] = "Float";
typeMap[Types.TYPE_INT64] = "Int64";
typeMap[Types.TYPE_UINT64] = "Uint64";
typeMap[Types.TYPE_INT32] = "Int32";
typeMap[Types.TYPE_FIXED64] = "Int64";
typeMap[Types.TYPE_FIXED32] = "Int32";
typeMap[Types.TYPE_BOOL] = "Bool";
typeMap[Types.TYPE_STRING] = "String";
typeMap[Types.TYPE_GROUP] = "Group";
typeMap[Types.TYPE_MESSAGE] = "Message";
typeMap[Types.TYPE_BYTES] = "NSData";
typeMap[Types.TYPE_UINT32] = "UInt32";
typeMap[Types.TYPE_ENUM] = "UInt32";
typeMap[Types.TYPE_SFIXED32] = "Int32";
typeMap[Types.TYPE_SFIXED64] = "Int64";
typeMap[Types.TYPE_SINT32] = "Int32";
typeMap[Types.TYPE_SINT64] = "Int64";

var protoTypeMap = {};
protoTypeMap[Types.TYPE_DOUBLE] = "Double";
protoTypeMap[Types.TYPE_FLOAT] = "Float";
protoTypeMap[Types.TYPE_INT64] = "Int64";
protoTypeMap[Types.TYPE_UINT64] = "Uint64";
protoTypeMap[Types.TYPE_INT32] = "Int32";
protoTypeMap[Types.TYPE_FIXED64] = "Fixed64";
protoTypeMap[Types.TYPE_FIXED32] = "Fixed32";
protoTypeMap[Types.TYPE_BOOL] = "Bool";
protoTypeMap[Types.TYPE_STRING] = "String";
protoTypeMap[Types.TYPE_GROUP] = "Group";
protoTypeMap[Types.TYPE_MESSAGE] = "Message";
protoTypeMap[Types.TYPE_BYTES] = "Data";
protoTypeMap[Types.TYPE_UINT32] = "Uint32";
protoTypeMap[Types.TYPE_ENUM] = "Enum";
protoTypeMap[Types.TYPE_SFIXED32] = "SFixed32";
protoTypeMap[Types.TYPE_SFIXED64] = "SFixed64";
protoTypeMap[Types.TYPE_SINT32] = "SInt32";
protoTypeMap[Types.TYPE_SINT64] = "SInt64";

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

function FixRef(ref) {
  return Type(ref.replace(/^.*\./, ''));
}

function Type(str) {
  return str;
}

function Var(str) {
  var name = str.split(/_/g).map(function (txt) {return txt.charAt(0).toUpperCase() + txt.substr(1).toLowerCase();}).join("")
  return name.charAt(0).toLowerCase() + name.substr(1);
}

function Const(str) {
  return str.split(/_/g).map(function (txt) {return txt.charAt(0).toUpperCase() + txt.substr(1).toLowerCase();}).join("")
}

function ComputeSizeCall(f) {
  var name = Var(f.Name);
  var compute = "compute" + protoTypeMap[f.Type] + "Size(" + f.Number + ")";
  if (f.Type == Types.TYPE_ENUM)
    compute = "rawValue." + compute
  if (f.Label == Labels.LABEL_REPEATED)
    return name + ".map({v in v." + compute + "}).reduce(0, +)"
  return name + (f.Label == Labels.LABEL_OPTIONAL ? "!" : "") + "." + compute;
}

function writeToOutput(f, name) {
  if (f.Type == Types.TYPE_ENUM)
    name += ".rawValue";
  return "output.write" + protoTypeMap[f.Type] + "(" + f.Number + ", value: " + name + ")";
}

function WriteFieldToOutput(f) {
  var name = Var(f.Name);
  if (f.Label == Labels.LABEL_REPEATED)
    return "for v in " + name + " { " + writeToOutput(f, "v") + " }";
  if (f.Label == Labels.LABEL_OPTIONAL)
    return "if " + name + " != nil { " + writeToOutput(f, name + "!") + " }";
  return writeToOutput(f, name);
}

function fieldTypeName(f) {
  return (f.Type == Types.TYPE_ENUM || f.Type == Types.TYPE_MESSAGE) ? FixRef(f.TypeName) : typeMap[f.Type];
}

function FieldDecl(f) {
  var name = Var(f.Name);
  var type = fieldTypeName(f);
  if (f.Label == Labels.LABEL_OPTIONAL)
    return name + ": " + type + "?"
  else if (f.Label == Labels.LABEL_REPEATED)
    return name + ": [" + type + "]"
  return name + ": " + type;
}

function OptionalFieldDecl(f) {
  var name = Var(f.Name);
  var type = fieldTypeName(f);
  if (f.Label == Labels.LABEL_REPEATED)
    return name + ": [" + type + "] = []"
  return name + ": " + type + "?";
}

function TypeDecl(t) {
  switch (t.Type) {
  case Types.TYPE_ENUM, Types.TYPE_MESSAGE:
    return FixRef(t.TypeName)

  default:
    return typeMap[t.Type];
  }
}

function TypeToSwift(t) {
  var name = t.Type == Types.TYPE_ENUM ? FixRef(t.TypeName) : typeMap[t.Type];
  return Type(name)  + (t.Label == Labels.LABEL_OPTIONAL ? "?" : "");
}

function TypeToProtocolBuffer(t) {
  return protoTypeMap[t.Type];
}

function FieldToVar(f) {
  return Var(f.Name) + (f.Label == Labels.LABEL_OPTIONAL ? "!" : "") + (f.Type == Types.TYPE_ENUM ? ".rawValue" : "");
}
