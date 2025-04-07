package docbook

import (
	"bufio"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"dario.cat/mergo"
	"github.com/beevik/etree"
	"github.com/mslacken/kowalski/internal/pkg/information"
)

type Docbook struct {
	Entities map[string]string
}

func ParseDocBook(filename string) (info information.Information, err error) {
	bk := Docbook{}
	return bk.ParseDocBook(filename)
}

func (bk *Docbook) ParseDocBook(filename string) (info information.Information, err error) {
	doc := etree.NewDocument()
	doc.ReadSettings = etree.ReadSettings{
		Entity: bk.Entities,
	}
	if err = doc.ReadFromFile(filename); err != nil {
		return
	}
	if root := doc.SelectElement("article"); root != nil {
		for _, section := range root.SelectElements("section") {
			subsec := parseElement(section)
			info.SubSections = append(info.SubSections, &subsec)
			if subsec.Title == "Environment" {
				info.OS = append(info.OS, subsec.Items...)
			}
		}
		// check for overall title
		if xmlInfo := root.SelectElement("info"); xmlInfo != nil {
			if elem := xmlInfo.SelectElement("title"); elem != nil {
				info.Title = elem.Text()
			}
		}
	}
	if info.Title == "" {
		info.Title = filepath.Base(filename)
	}
	return
}

func parseElement(elem *etree.Element) (info information.Section) {
	{
		str := strings.TrimSpace(strings.ReplaceAll(html.UnescapeString(elem.Text()), "\n", ""))
		info.Text = str
	}
	for _, child := range elem.ChildElements() {
		subinfo := parseElement(child)
		strChild := subinfo.Text
		switch child.Tag {
		case "title", "Title":
			info.Title = subinfo.Text
		case "para":
			info.Text += subinfo.Text
		case "literal", "replaceable":
			str := strings.TrimSpace(strings.ReplaceAll(html.UnescapeString(child.Text()), "\n", ""))
			info.Text += " `" + str + "` "
			info.Text += child.Tail()
		case "listitem":
			info.Items = append(info.Items, strChild)
		case "varlistentry":
			newItem := strings.Join(subinfo.Items, " ") + strChild
			info.Items = append(info.Items, newItem)
		case "term", "screen":
			info.Text += strChild
		case "command":
			info.Text += "```" + strChild + "``` "
			info.Commands = []string{subinfo.Text}
		default:
			subinfo := parseElement(child)
			if info.Title == "Environment" {
				if err := mergo.Merge(&info, subinfo); err != nil {
					panic("couldn't merge during parsing")
				}
			} else {
				info.SubSections = append(info.SubSections, &subinfo)
			}
		}
		info.Commands = append(info.Commands, subinfo.Commands...)
	}
	return
}

func ReadEntity(filename string) (entities map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	entityRegex := regexp.MustCompile(`<!ENTITY\s+([\p{L}][^\s]+)\s+"([^"]+)"\s*>`)

	entities = make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		match := entityRegex.FindStringSubmatch(line)
		if len(match) == 3 {
			if match[1] != "" && match[2] != "" {
				entities[match[1]] = match[2]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return entities, nil
}
