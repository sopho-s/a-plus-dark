package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	args := os.Args
	printlex := false
	nosyntax := false
	inputfile := ""
	outputfile := ""
	verbose := false
	for index, val := range args {
		switch val {
		case "-lex":
			printlex = true
			break
		case "-v":
			verbose = true
			break
		case "-dev":
			printlex = true
			verbose = true
			break
		case "-olive":
			nosyntax = true
			break
		case "-in":
			if len(args)-1 == index {
				fmt.Println("No input file specified with -o")
				return
			}
			inputfile = args[index+1]
			break
		case "-out":
			if len(args)-1 == index {
				fmt.Println("No output file specified with -o")
				return
			}
			outputfile = args[index+1]
			break
		}
	}
	var compilationlog buildlog
	_ = os.Mkdir("build", 0755)
	RemoveComments(inputfile)
	compilationlog.AddLog("Removing comments", 0)
	if verbose {
		fmt.Println("Removing comments")
	}

	compilationlog.AddLog("Performing lexical analysis on file", 0)
	if verbose {
		fmt.Println("Performing lexical analysis on file")
	}
	nodes, includes := LexicalAnalysis(printlex)
	compilationlog.AddLog("Lexical analysis finished", 0)
	if verbose {
		fmt.Println("Lexical analysis finished")
	}
	if len(includes) > 0 {
		includelog := "Includes found: "
		for _, val := range includes {
			includelog += val + ", "
		}
		includelog = includelog[:len(includelog)-2]
		compilationlog.AddLog(includelog, 0)
	}
	compilationlog.AddLog("Seperating functions", 0)
	if verbose {
		fmt.Println("Seperating functions")
	}
	functions, shouldcontinue := SeperateFunctions(nodes, &compilationlog)
	compilationlog.AddLog("Functions seperated", 0)
	if verbose {
		fmt.Println("Functions seperated")
	}
	if !shouldcontinue {
		fmt.Println("Compilation errors occured")
		compilationlog.AddLog("Compilation errors occured", 2)
		f, _ := os.Create("log/buildlog.log")
		f.WriteString(compilationlog.ReturnLogs())
		return
	}
	var datacode code
	var funccode code
	var df definedfunctions
	df.functions = make(map[string]nodefunction)
	offsetmap := make(map[string]int)
	floatcountmap := make(map[string]int)
	for _, function := range functions {
		function.RemoveStartAndEnd()
		nodes = function.nodes
		var vl variablelist
		for _, vari := range function.parameters {
			vl.Add(vari.variable)
		}
		compilationlog.AddLog("Performing syntax analysis on \""+function.name+"\"", 0)
		if verbose {
			fmt.Println("Performing syntax analysis on \"" + function.name + "\"")
		}
		nodes, shouldcontinue, compilationlog = SyntaxAnalysis(nodes, compilationlog, nosyntax, vl, df)
		compilationlog.AddLog("Syntax analysis finished", 0)
		if verbose {
			fmt.Println("Syntax analysis finished")
		}

		if !shouldcontinue {
			fmt.Println("Compilation errors occured")
			compilationlog.AddLog("Compilation errors occured", 2)
			f, _ := os.Create("log/buildlog.log")
			f.WriteString(compilationlog.ReturnLogs())
			return
		}

		compilationlog.AddLog("Converting \""+function.name+"\" to postfix", 0)
		if verbose {
			fmt.Println("Converting \"" + function.name + "\" to postfix")
		}
		outqueue := MakePostfix(nodes)
		compilationlog.AddLog("Postfix conversion done", 0)
		if verbose {
			fmt.Println("Postfix conversion done")
		}

		compilationlog.AddLog("Making abstract syntax tree for \""+function.name+"\"", 0)
		if verbose {
			fmt.Println("Making abstract syntax tree for \"" + function.name + "\"")
		}
		AST := ConvertPostfix(outqueue)
		compilationlog.AddLog("Abstact syntax tree is done", 0)
		if verbose {
			fmt.Println("Abstact syntax tree is done")
		}

		var outcode code
		compilationlog.AddLog("Making intermediate code for \""+function.name+"\"", 0)
		if verbose {
			fmt.Println("Making intermediate code for \"" + function.name + "\"")
		}
		for _, node := range AST.children {
			newcode, _ := MakeIntermediate(node)
			outcode.AddCode(newcode)
		}
		compilationlog.AddLog("Intermediate code created", 0)
		if verbose {
			fmt.Println("Intermediate code created")
		}

		f, _ := os.Create("build/" + function.name + ".preint")
		f.WriteString(outcode.store)

		f, _ = os.Create("build/" + function.name + ".postint")
		f.WriteString(OptimiseIntermediate(outcode.store))

		f, _ = os.Create("build/" + function.name + ".asm")
		compilationlog.AddLog("Optimising and converting intermediate code into assembly for \""+function.name+"\"", 0)
		if verbose {
			fmt.Println("Optimising and converting intermediate code into assembly for \"" + function.name + "\"")
		}
		writecode, predatacode, logcode := ConvertToNASM(OptimiseIntermediate(outcode.store), function.name, &offsetmap, &floatcountmap)
		datacode.AddCode(predatacode)
		compilationlog.AddLog("Assembly created", 0)
		if verbose {
			fmt.Println("Assembly created")
		}
		f.WriteString(writecode.store)
		file, _ := os.ReadFile("build/" + inputfile)
		SetLoggingConversion(string(file), logcode)
		_ = os.Mkdir("log", 0755)
		f, _ = os.Create("log/" + function.name + "codelog.log")
		for index, _ := range logcode {
			f.WriteString(logcode[index].originalcode.store)
			f.WriteString("\nWas converted to:\n")
			f.WriteString(logcode[index].assemblycode.store)
			f.WriteString("\n\n\n")
		}
		funccode.AddCode(writecode)
		df.AddFunction(function)
	}
	f, _ := os.Create("build/build.asm")
	compilationlog.AddLog("", 0)

	compilationlog.AddLog("Making build assembly", 0)
	if verbose {
		fmt.Println("Making build assembly")
	}
	buildcode := AddingPrecodeToFunctions(funccode, datacode)
	compilationlog.AddLog("Made build assembly", 0)
	if verbose {
		fmt.Println("Made build assembly")
	}
	f.WriteString(buildcode.store)

	compilationlog.AddLog("Linking imports", 0)
	if verbose {
		fmt.Println("Linking imports")
	}
	Link(includes, "build.asm", &compilationlog)
	compilationlog.AddLog("Imports linked", 0)
	if verbose {
		fmt.Println("Imports linked")
	}

	compilationlog.AddLog("Compiling to object file", 0)
	if verbose {
		fmt.Println("Compiling to object file")
	}
	var stderr buf
	cmd := exec.Command("nasm", "-f", "win32", "build/build.asm", "-o", "build/build.obj")
	cmd.Stderr = &stderr
	_, err := cmd.Output()
	if err != nil {
		fmt.Println("Error in compilation to object file")
		compilationlog.AddLog("Error in compilation to object file", 2)
		errorstring := fmt.Sprint(stderr.String())
		spliterr := strings.Split(errorstring, "\r\n")
		for index, val := range spliterr {
			if index != len(spliterr)-1 {
				compilationlog.AddLog(val, 2)
			}
		}
		f, _ := os.Create("log/buildlog.log")
		f.WriteString(compilationlog.ReturnLogs())
		return
	}
	compilationlog.AddLog("Object file compiled", 0)
	if verbose {
		fmt.Println("Object file compiled")
	}

	compilationlog.AddLog("Compiling to executable", 0)
	if verbose {
		fmt.Println("Compiling to executable")
	}
	cmd = exec.Command("gcc", "-Wall", "-Wextra", "-o", "build/"+outputfile, "build/build.obj")
	cmd.Stderr = &stderr
	_, err = cmd.Output()

	if err != nil {
		fmt.Println("Error in compilation to executable")
		compilationlog.AddLog("Error in compilation to executable", 2)
		errorstring := fmt.Sprint(stderr.String())
		spliterr := strings.Split(errorstring, "\n")
		for index, val := range spliterr {
			if index != len(spliterr)-1 {
				compilationlog.AddLog(val, 2)
			}
		}
		f, _ := os.Create("log/buildlog.log")
		f.WriteString(compilationlog.ReturnLogs())
		return
	}
	compilationlog.AddLog("Executable compiled", 0)
	if verbose {
		fmt.Println("Executable compiled")
	}
	f, _ = os.Create("log/buildlog.log")
	f.WriteString(compilationlog.ReturnLogs())
}
