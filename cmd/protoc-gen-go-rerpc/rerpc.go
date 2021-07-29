package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/akshayjshah/rerpc"
)

const (
	contextPackage = protogen.GoImportPath("context")
	rerpcPackage   = protogen.GoImportPath("github.com/akshayjshah/rerpc")
	httpPackage    = protogen.GoImportPath("net/http")
	protoPackage   = protogen.GoImportPath("google.golang.org/protobuf/proto")
	stringsPackage = protogen.GoImportPath("strings")
)

func deprecated(g *protogen.GeneratedFile) {
	comment(g, "// Deprecated: do not use.")
}

func generate(gen *protogen.Plugin, file *protogen.File) *protogen.GeneratedFile {
	if len(file.Services) == 0 {
		return nil
	}
	filename := file.GeneratedFilenamePrefix + "_rerpc.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	preamble(gen, file, g)
	content(file, g)
	return g
}

func protocVersion(gen *protogen.Plugin) string {
	v := gen.Request.GetCompilerVersion()
	if v == nil {
		return "(unknown)"
	}
	out := fmt.Sprintf("v%d.%d.%d", v.GetMajor(), v.GetMinor(), v.GetPatch())
	if s := v.GetSuffix(); s != "" {
		out += "-" + s
	}
	return out
}

func preamble(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile) {
	g.P("// Code generated by protoc-gen-go-rerpc. DO NOT EDIT.")
	g.P("// versions:")
	g.P("// - protoc-gen-go-rerpc v", rerpc.Version)
	g.P("// - protoc             ", protocVersion(gen))
	if file.Proto.GetOptions().GetDeprecated() {
		comment(g, file.Desc.Path(), " is a deprecated file.")
	} else {
		g.P("// source: ", file.Desc.Path())
	}
	g.P()
	g.P("package ", file.GoPackageName)
	g.P()
}

func content(file *protogen.File, g *protogen.GeneratedFile) {
	if len(file.Services) == 0 {
		return
	}
	handshake(g)
	for _, svc := range file.Services {
		service(file, g, svc)
	}
}

func handshake(g *protogen.GeneratedFile) {
	comment(g, "This is a compile-time assertion to ensure that this generated file ",
		"and the rerpc package are compatible. If you get a compiler error that this constant ",
		"isn't defined, this code was generated with a version of rerpc newer than the one ",
		"compiled into your binary. You can fix the problem by either regenerating this code ",
		"with an older version of rerpc or updating the rerpc version compiled into your binary.")
	g.P("const _ = ", rerpcPackage.Ident("SupportsCodeGenV0"), " // requires reRPC v0.0.1 or later")
	g.P()
}

func service(file *protogen.File, g *protogen.GeneratedFile, service *protogen.Service) {
	clientName := service.GoName + "ClientReRPC"
	serverName := service.GoName + "ServerReRPC"

	clientInterface(g, service, clientName)
	clientImplementation(g, service, clientName)
	serverInterface(g, service, serverName)
	serverConstructor(g, service, serverName)
	serverImplementation(g, service, serverName)
}

func clientInterface(g *protogen.GeneratedFile, service *protogen.Service, name string) {
	comment(g, name, " is a client for the ", service.Desc.FullName(), " service.")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		deprecated(g)
	}
	g.Annotate(name, service.Location)
	g.P("type ", name, " interface {")
	for _, method := range unaryMethods(service) {
		g.Annotate(name+"."+method.GoName, method.Location)
		g.P(method.Comments.Leading, clientSignature(g, method))
	}
	g.P("}")
	g.P()
}

func clientSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
		deprecated(g)
	}
	return method.GoName + "(ctx " + g.QualifiedGoIdent(contextPackage.Ident("Context")) +
		", req *" + g.QualifiedGoIdent(method.Input.GoIdent) +
		", opts ..." + g.QualifiedGoIdent(rerpcPackage.Ident("CallOption")) + ") " +
		"(*" + g.QualifiedGoIdent(method.Output.GoIdent) + ", error)"
}

func clientImplementation(g *protogen.GeneratedFile, service *protogen.Service, name string) {
	// Client struct.
	g.P("type ", unexport(name), " struct {")
	for _, method := range unaryMethods(service) {
		g.P(unexport(method.GoName), " ", rerpcPackage.Ident("Client"))
	}
	g.P("}")
	g.P()

	// Client constructor.
	comment(g, "New", name, " constructs a client for the ", service.Desc.FullName(),
		" service. Call options passed here apply to all calls made with this client.")
	g.P("//")
	comment(g, "The URL supplied here should be the base URL for the gRPC server ",
		"(e.g., https://api.acme.com or https://acme.com/api/grpc).")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		deprecated(g)
	}
	g.P("func New", name, " (baseURL string, doer ", rerpcPackage.Ident("Doer"),
		", opts ...", rerpcPackage.Ident("CallOption"), ") ", name, " {")
	g.P("baseURL = ", stringsPackage.Ident("TrimRight"), `(baseURL, "/")`)
	g.P("return &", unexport(name), "{")
	for _, method := range unaryMethods(service) {
		path := fmt.Sprintf("%s/%s", service.Desc.FullName(), method.Desc.Name())
		g.P(unexport(method.GoName), ": *", rerpcPackage.Ident("NewClient"), "(")
		g.P("doer,")
		g.P(`baseURL + "/`, path, `", // complete URL to call method`)
		g.P(`"`, method.Desc.FullName(), `", // fully-qualified protobuf method`)
		g.P(`"`, service.Desc.FullName(), `", // fully-qualified protobuf service`)
		g.P(`"`, service.Desc.ParentFile().Package(), `", // fully-qualified protobuf package`)
		g.P("opts...,")
		g.P("),")
	}
	g.P("}")
	g.P("}")
	g.P()

	// Client method implementations.
	for _, method := range unaryMethods(service) {
		clientMethod(g, method)
	}
}

func clientMethod(g *protogen.GeneratedFile, method *protogen.Method) {
	comment(g, method.GoName, " calls ", method.Desc.FullName(), ".",
		" Call options passed here apply only to this call.")
	if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
		g.P("//")
		deprecated(g)
	}
	g.P("func (c *", unexport(method.Parent.GoName), "ClientReRPC) ", clientSignature(g, method), "{")
	g.P("res := &", method.Output.GoIdent, "{}")
	g.P("if err := c.", unexport(method.GoName), ".Call(ctx, req, res, opts...); err != nil {")
	g.P("return nil, err")
	g.P("}")
	g.P("return res, nil")
	g.P("}")
	g.P()
}

func serverInterface(g *protogen.GeneratedFile, service *protogen.Service, name string) {
	comment(g, name, " is a server for the ", service.Desc.FullName(),
		" service. To make sure that adding methods to this protobuf service doesn't break all ",
		"implementations of this interface, all implementations must embed Unimplemented",
		name, ".")
	g.P("//")
	comment(g, "By default, recent versions of grpc-go have a similar forward compatibility ",
		"requirement. See https://github.com/grpc/grpc-go/issues/3794 for a longer discussion.")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		deprecated(g)
	}
	g.Annotate(name, service.Location)
	g.P("type ", name, " interface {")
	for _, method := range unaryMethods(service) {
		g.Annotate(name+"."+method.GoName, method.Location)
		g.P(method.Comments.Leading, serverSignature(g, method))
	}
	g.P("mustEmbedUnimplemented", name, "()")
	g.P("}")
	g.P()
}

func serverSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	if method.Desc.Options().(*descriptorpb.MethodOptions).GetDeprecated() {
		deprecated(g)
	}
	return method.GoName + "(" + g.QualifiedGoIdent(contextPackage.Ident("Context")) +
		", *" + g.QualifiedGoIdent(method.Input.GoIdent) + ") " +
		"(*" + g.QualifiedGoIdent(method.Output.GoIdent) + ", error)"
}

func serverConstructor(g *protogen.GeneratedFile, service *protogen.Service, name string) {
	sname := service.Desc.FullName()
	comment(g, "New", service.GoName, "HandlerReRPC wraps the service implementation",
		" in an HTTP handler. It returns the handler and the path on which to mount it.")
	if service.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		deprecated(g)
	}
	g.P("func New", service.GoName, "HandlerReRPC(svc ", name, ", opts ...", rerpcPackage.Ident("HandlerOption"),
		") (string, ", httpPackage.Ident("Handler"), ") {")
	g.P("mux := ", httpPackage.Ident("NewServeMux"), "()")
	g.P()
	for _, method := range unaryMethods(service) {
		path := fmt.Sprintf("%s/%s", sname, method.Desc.Name())
		hname := unexport(string(method.Desc.Name()))
		g.P(hname, " := ", rerpcPackage.Ident("NewHandler"), "(")
		g.P(`"`, method.Desc.FullName(), `", // fully-qualified protobuf method`)
		g.P(`"`, service.Desc.FullName(), `", // fully-qualified protobuf service`)
		g.P(`"`, service.Desc.ParentFile().Package(), `", // fully-qualified protobuf package`)
		g.P(rerpcPackage.Ident("UnaryHandler"), "(func(ctx ", contextPackage.Ident("Context"),
			", req ", protoPackage.Ident("Message"), ") (",
			protoPackage.Ident("Message"), ", error) {")
		g.P("typed, ok := req.(*", method.Input.GoIdent, ")")
		g.P("if !ok {")
		g.P("return nil, ", rerpcPackage.Ident("Errorf"), "(")
		g.P(rerpcPackage.Ident("CodeInternal"), ",")
		g.P(`"error in generated code: expected req to be a *`, method.Input.GoIdent, `, got a %T",`)
		g.P("req,")
		g.P(")")
		g.P("}")
		g.P("return svc.", method.GoName, "(ctx, typed)")
		g.P("}),")
		g.P("opts...,")
		g.P(")")
		g.P(`mux.HandleFunc("/`, path, `", func(w `, httpPackage.Ident("ResponseWriter"), ", r *", httpPackage.Ident("Request"), ") {")
		g.P(hname, ".Serve(w, r, &", method.Input.GoIdent, "{})")
		g.P("})")
		g.P()
	}
	g.P(`return "/`, sname, `/", mux`)
	g.P("}")
	g.P()
}

func serverImplementation(g *protogen.GeneratedFile, service *protogen.Service, name string) {
	g.P("var _ ", name, " = (*Unimplemented", name, ")(nil) // verify interface implementation")
	g.P()
	// Unimplemented server implementation (for forward compatibility).
	comment(g, "Unimplemented", name, " returns CodeUnimplemented from",
		" all methods. To maintain forward compatibility, all implementations",
		" of ", name, " must embed Unimplemented", name, ". ")
	g.P("type Unimplemented", name, " struct {}")
	g.P()
	for _, method := range unaryMethods(service) {
		g.P("func (Unimplemented", name, ") ", serverSignature(g, method), "{")
		g.P("return nil, ", rerpcPackage.Ident("Errorf"), "(", rerpcPackage.Ident("CodeUnimplemented"), `, "method `, method.GoName, ` not implemented")`)
		g.P("}")
		g.P()
	}
	g.P("func (Unimplemented", name, ") mustEmbedUnimplemented", name, "() {}")
	g.P()
}

func unexport(s string) string { return strings.ToLower(s[:1]) + s[1:] }

func unaryMethods(service *protogen.Service) []*protogen.Method {
	unary := make([]*protogen.Method, 0, len(service.Methods))
	for _, m := range service.Methods {
		if m.Desc.IsStreamingServer() || m.Desc.IsStreamingClient() {
			continue
		}
		unary = append(unary, m)
	}
	return unary
}
