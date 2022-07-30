package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func splitFile() {

	var folder = "D:\\projects\\go\\temp\\ncbiasn"
	var package_prefix = "ncbiasn/"
	file1, err := os.Open("D:\\projects\\go\\asn1\\asn1go\\cmd\\asn1go\\test3.go.txt")

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer file1.Close()

	bufReader := bufio.NewReader(file1)
	txt := ""
	str := ""
	filename := ""
	for {

		line, isPrefix, err := bufReader.ReadLine()
		if err != nil {
			break
		}

		if isPrefix {
			str += string(line)
		} else {
			str = string(line)
			if strings.HasPrefix(str, "import") {
				words3 := strings.Fields(str)
				if len(words3) > 1 {
					package2 := words3[1]
					package2 = strings.Trim(package2, "\"")
					package2 = package_prefix + package2
					str = fmt.Sprintf("import \"%s\"", package2)
				}

			}

			if strings.HasPrefix(str, "package") {
				if txt != "" {

					err1 := os.WriteFile(filename, []byte(txt), os.ModePerm)
					if err1 != nil {
						fmt.Println(err.Error())
						continue
					}

					// fmt.Println(txt)

				}
				txt = str
				words := strings.Fields(str)
				if len(words) > 1 {
					package1 := words[1]
					fmt.Println(package1)
					fullpath := fmt.Sprintf("%s/%s", folder, package1)
					err1 := os.MkdirAll(fullpath, os.ModePerm)
					if err1 != nil {
						fmt.Println(err.Error())
						continue
					}
					filename = fmt.Sprintf("%s/module.go", fullpath)
				}
			} else {
				txt += str + "\n"
			}

			str = ""
		}

	}

}

func ChangeNumberCase() {

	caser := cases.Title(language.English, cases.NoLower)
	lstr := caser.String("presentInChildCD")
	fmt.Println(lstr)

	lstr = strings.Title(strings.Replace("presentInChildCD", "-", "_", -1))
	fmt.Println(lstr)
}
