package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
)

type RejectionReasonStruct struct {
	XMLName xml.Name `xml:"rejectionReason"`
	Name    string   `xml:"name"`
}

type CriterionStruct struct {
	Name            string `xml:"name"`
	NegativeMeaning string `xml:"negativeMeaning"`
}

func openDictionary(dictionary, service string) (*os.File, string, error) {
	specific_dict := fmt.Sprintf("%s_%s.xml", service, dictionary)
	general_dict := fmt.Sprintf("%s.xml", dictionary)
	dict, err := os.Open("./dictionaries/" + specific_dict)
	if err != nil {
		if _, ok := err.(*fs.PathError); ok {
			// add output here that wasn't found specific dict for this service and used general one
			//fmt.Printf("%s (path \"%s\")\n", pathErr.Err, pathErr.Path)
			dict, err = os.Open("./dictionaries/" + general_dict)
			if err != nil {
				return nil, general_dict, fmt.Errorf("%s.xml dictionary opening failure: %v", dictionary, err)
			}
			return dict, general_dict, nil
		} else {
			return nil, specific_dict, fmt.Errorf("%s_%s.xml dictionary opening failure: %v", service, dictionary, err)
		}
	}
	return dict, specific_dict, nil
}

func decodeRejectReasonsFromDict(dict *os.File, rejections map[string]string) error {
	decoder := xml.NewDecoder(dict)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "record" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "recordVersionId" {
						var name string
						decoder.DecodeElement(&name, &t)
						//fmt.Printf("elem: %s\n", name)

						var data map[string]interface{}
						err := json.Unmarshal([]byte(name), &data)
						if err != nil {
							// return to output file info about that
							return fmt.Errorf("json parsing failure of %s: %v", dict.Name(), err)
						}

						if data["name"] == nil {
							continue
						}

						if _, ok := rejections[data["name"].(string)]; ok {
							rejections[data["name"].(string)] = attr.Value
							fmt.Printf("found reason record %s\n", attr.Value)
						}
					}
				}
			}
		}
	}
	return nil
}

func decodeNegativeMeaningsFromDict(dict *os.File, negMeanings map[CriterionStruct]string) error {
	decoder := xml.NewDecoder(dict)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "record" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "recordVersionId" {
						var jsonStr string
						decoder.DecodeElement(&jsonStr, &t)
						//fmt.Printf("elem: %s\n", name)

						var data map[string]interface{}
						err := json.Unmarshal([]byte(jsonStr), &data)
						if err != nil {
							// return to output file info about that
							return fmt.Errorf("json parsing failure of %s: %v", dict.Name(), err)
						}

						if data["negativeMeaning"] == nil || data["name"] == nil {
							continue
						}

						obj := CriterionStruct{Name: data["name"].(string), NegativeMeaning: data["negativeMeaning"].(string)}

						if _, ok := negMeanings[obj]; ok {
							negMeanings[obj] = attr.Value
							//fmt.Printf("found negMeaning record %s for '%.190s'\n", attr.Value, data["negativeMeaning"].(string))
						}
					}
				}
			}
		}
	}
	return nil
}

func main() {
	logFile, err := os.Create("log")
	if err != nil {
		log.Printf("couldn't create log file: %v", err)
	}
	fileSystem := os.DirFS(".")

	indent := "    "

	text := `do
$$
declare
	rec record;
	key_type int8;
begin
	for rec in (select scheme from regadm.m_projects where scheme = 'kostgo') loop
		perform set_config('search_path', rec.scheme, true);
		raise info '%', rec.scheme;

		select key into key_type from d_ref_dependent_type where full_name like 'DocRefRejectReasonType';
		raise info 'key of DocRefRejectReasonType = %', key_type;
`

	fs.WalkDir(fileSystem, "reglaments", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logFile.Write([]byte("couldn't open reglaments dir: " + err.Error()))
			os.Exit(1)
		}
		if d.IsDir() {
			return nil
		}
		fmt.Println(path)

		service := strings.Split(d.Name(), ".")[0]
		fmt.Println(service)

		reglament, err := os.Open(path)
		if err != nil {
			logFile.Write([]byte(fmt.Sprintf("couldn't open reglament at path \"%s\": %v", path, err)))
			return nil
		}
		defer reglament.Close()

		decoder := xml.NewDecoder(reglament)

		rejections := make(map[string]string)
		negMeanings := make(map[CriterionStruct]string)

		var rejectionReason string
		var crit CriterionStruct

		for {
			token, err := decoder.Token()
			if err != nil {
				break
			}

			switch t := token.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "criterion":
					err = decoder.DecodeElement(&crit, &t)
					if err != nil {
						logFile.Write([]byte(fmt.Sprintf("criterion parsing failure in reglament at path \"%s\": %v", path, err)))
						return nil
					}
					//fmt.Printf("%v\n", crit)
				case "rejectionReason":
					var rej RejectionReasonStruct
					err = decoder.DecodeElement(&rej, &t)
					if err != nil {
						logFile.Write([]byte(fmt.Sprintf("rejectionReason parsing failure in reglament at path \"%s\": %v", path, err)))
						return nil
					}
					rejectionReason = rej.Name
					//fmt.Printf("%v\n", criteria)
				}
			case xml.EndElement:
				if t.Name.Local == "criteria" {
					//fmt.Printf("%s ------ %s\n", crit.Name, crit.NegativeMeaning)
					if rejectionReason != "" {
						if _, ok := rejections[rejectionReason]; !ok {
							rejections[rejectionReason] = "TRASH"
						}
					} else if crit.NegativeMeaning != "" {
						//fmt.Printf("%s ------ %s\n", crit.Name, crit.NegativeMeaning)
						if _, ok := negMeanings[crit]; !ok {
							negMeanings[crit] = "TRASH"
						}
					}

					rejectionReason = ""
					crit = CriterionStruct{}
				}
			}
		}

		rejectReasonDict, usedRejDict, err := openDictionary("rejectReason", service)
		if err != nil {
			// add to output file
			logFile.Write([]byte(fmt.Sprintf("couldn't open rejectReason dictionary: %v", err)))
		} else {
			defer rejectReasonDict.Close()
			err = decodeRejectReasonsFromDict(rejectReasonDict, rejections)
			if err != nil {
				//write to output file error message
				logFile.Write([]byte(fmt.Sprintf("couldn't decode rejectReason dictionary: %v", err)))
			}
		}

		negativeMeaningDict, usedReasonDict, err := openDictionary("reasonForSuccessDecision", service)
		if err != nil {
			// add to output file
			logFile.Write([]byte(fmt.Sprintf("couldn't open reasonForSuccessDecision dictionary: %v", err)))
		} else {
			defer negativeMeaningDict.Close()
			err = decodeNegativeMeaningsFromDict(negativeMeaningDict, negMeanings)
			if err != nil {
				//write to output file error message
				logFile.Write([]byte(fmt.Sprintf("couldn't decode reasonForSuccessDecision dictionary: %v", err)))
			}
		}

		i := 0
		if len(rejections) != 0 {
			text += "\n" + indent[:2] + "--Used " + usedRejDict
			for rejectName, rejectId := range rejections {
				i++
				if len([]rune(rejectName)) >= 1000 {
					text += fmt.Sprintf("\n%s--%d characters in full_name, could be limit in db on that column in 1000 symbols",
						indent, len([]rune(rejectName)))
				}
				if rejectId == "TRASH" {
					text += fmt.Sprintf("\n%s--couldn't find GUID myself, so you need to do that using name in dictionary", indent)
				}

				if !strings.HasSuffix(text, ";") && !strings.HasSuffix(text, "xml") {
					text += "\n" + indent + "  --"
				} else {
					text += "\n" + indent
				}
				text += fmt.Sprintf("insert into d_ref_dependents (alias, code_kcr, dependent_type, full_name, name, is_draft, sys_status) values ('Opal%sRejReason%d', '%s', key_type, '%s','%s', 0, 0);",
					service, i, rejectId, rejectName, rejectName)
			}
			text += "\n"
		}
		if len(negMeanings) != 0 {
			text += "\n" + indent[:2] + "--Used " + usedReasonDict
			for obj, negMeanId := range negMeanings {
				i++
				if len([]rune(obj.NegativeMeaning)) >= 1000 {
					text += fmt.Sprintf("\n%s-- %d characters in full_name, could be limit in db on that column in 1000 symbols",
						indent, len([]rune(obj.NegativeMeaning)))
				}
				if negMeanId == "TRASH" {
					text += fmt.Sprintf("\n%s--couldn't find GUID myself, so you need to do that using name in dictionary", indent)
				}

				if !strings.HasSuffix(text, ";") && !strings.HasSuffix(text, "xml") {
					text += "\n" + indent + "  --"
				} else {
					text += "\n" + indent
				}
				text += fmt.Sprintf("insert into d_ref_dependents (alias, code_kcr, dependent_type, full_name, name, is_draft, sys_status) values ('Opal%sRejMeaning%d', '%s', key_type, '%s','%s', 0, 0);",
					service, i, negMeanId, obj.NegativeMeaning, obj.NegativeMeaning)
			}
		}
		text += "\n\n" + indent[:4] + "-----------------------------------------"

		return nil
	})
	text += "\n" + `    end loop;
end;
$$;`

	text += "\n\n\n" + `
--запрос с фильтром для проверки созданных отказов на схеме
select * from <scheme>.d_ref_dependents where alias like "Opal%";


--чистка бд от созданных для опала отказов
do
$$
declare
	rec record;
	key_type int8;
begin
	for rec in (select scheme from regadm.m_projects where scheme = 'kostgo') loop
		perform set_config('search_path', rec.scheme, true);
		raise info '%', rec.scheme;

		delete from d_ref_dependents where alias like "Opal%";
	end loop;
end;
$$;`

	outputFile, err := os.Create("script_to_add_rejection_reasons_for_opal.sql")
	if err != nil {
		log.Fatalf("output creation failure: %v", err)
	}
	defer outputFile.Close()
	_, err = outputFile.Write([]byte(text))
	if err != nil {
		log.Fatalf("writing output failure: %v", err)
	}
	//fmt.Println(text)
}

/*
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
<soap:Body>
<ns2:GetResponseResponse xmlns="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/basic/1.2" xmlns:ns2="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/1.2" xmlns:ns3="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/faults/1.2">
<ns2:ResponseMessage>
<ns2:Response xmlns:ns2="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/1.2" Id="SIGNED_BY_SMEV">
<SenderProvidedResponseData xmlns="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/1.2" xmlns:S="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns2="urn://x-artefacts-smev-gov-ru/services/message-exchange/types/basic/1.2" xmlns:ns3="http://www.w3.org/2004/08/xop/include" Id="SIGNED_BY_CONSUMER">
<ns2:MessagePrimaryContent>
<tns:KCRResponse xmlns:tns="urn://rostelekom.ru/KCR/1.0.6">
<tns:kcrService>
<tns:secondaryFields>
<tns:subServices>
<tns:profiling>
<tns:profilingSettings>
<tns:answers>
<tns:additionalInformations>
<tns:criteria>
<tns:rejectionReason>
<tns:id>
chardate here
*/
