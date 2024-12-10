package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
)

type RejectionReasonStruct struct {
	XMLName xml.Name `xml:"rejectionReason"`
	Name    string   `xml:"name"`
}

type CriterionStruct struct {
	//XMLName         xml.Name `xml:"criterion"`
	Name            string `xml:"name"`
	NegativeMeaning string `xml:"negativeMeaning"`
}

func main() {
	service := "RS"

	reglament, err := os.Open("./reglaments/" + service + ".xml")
	if err != nil {
		log.Fatalf("reglament opening failure: %v", err)
	}
	defer reglament.Close()

	decoder := xml.NewDecoder(reglament)

	rejections := make(map[string]string, 0)
	negMeanings := make(map[CriterionStruct]string, 0)

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
					log.Fatalf("criterion parsing failure: %v", err)
				}
				//fmt.Printf("%v\n", crit)
			case "rejectionReason":
				var rej RejectionReasonStruct
				err = decoder.DecodeElement(&rej, &t)
				if err != nil {
					log.Fatalf("rejectionReason parsing failure: %v", err)
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
		/*
			switch t := token.(type) {
			case xml.StartElement:
				if t.Name.Local == "rejectionReason" {
					exit := false
					for !exit {
						token2, err := decoder.Token()
						if err != nil {
							break
						}
						switch t2 := token2.(type) {
						case xml.StartElement:
							if t2.Name.Local != "id" {
								exit = true
							}

						case xml.CharData:
							if str_t := strings.TrimSpace(string(t2)); str_t != "" {
								rejections[str_t] = "TRASH"
								exit = true
							}
						}
					}
				}
			}
		*/
	}

	var rejectReasonDict *os.File
	if service == "RS" {
		rejectReasonDict, err = os.Open("./dictionaries/rejectReasonForRS.xml")
	} else {
		rejectReasonDict, err = os.Open("./dictionaries/rejectReason.xml")
	}
	if err != nil {
		log.Fatalf("rejectReason dictionary opening failure: %v", err)
	}
	defer rejectReasonDict.Close()

	decoder = xml.NewDecoder(rejectReasonDict)
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
							log.Fatalf("json parsing failure: %v", err)
						}

						if data["name"] == nil {
							continue
						}

						if _, ok := rejections[data["name"].(string)]; ok {
							rejections[data["name"].(string)] = attr.Value
							fmt.Printf("found reason record %s\n", attr.Value)
							/*
								exit := false
								for !exit {
									token2, err := decoder.Token()
									if err != nil {
										break
									}
									switch t2 := token2.(type) {
									case xml.CharData:
										//fmt.Printf("attrs ---- %s\n", strings.TrimSpace(string(t2)))
										if str_t := strings.TrimSpace(string(t2)); str_t != "" {
											//fmt.Printf("attrs ---- %s\n", str_t)

											var data map[string]interface{}

											err := json.Unmarshal([]byte(str_t), &data)
											if err != nil {
												log.Fatalf("json parsing failure: %v", err)
											}

											rejections[attr.Value] = data["name"].(string)
											exit = true
										}
									}
								}
							*/
						}
					}
				}
			}
		}
	}

	var negativeMeaningDict *os.File
	if service == "RS" {
		negativeMeaningDict, err = os.Open("./dictionaries/reasonForSuccessDecisionForRS.xml")
	} else {
		negativeMeaningDict, err = os.Open("./dictionaries/reasonForSuccessDecision.xml")
	}
	if err != nil {
		log.Fatalf("reasonForSuccessDecision dictionary opening failure: %v", err)
	}
	defer negativeMeaningDict.Close()

	temp, err := os.Create("temp")
	if err != nil {
		log.Fatalf("output creation failure: %v", err)
	}
	defer temp.Close()

	decoder = xml.NewDecoder(negativeMeaningDict)
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
							log.Fatalf("json parsing failure: %v", err)
						}

						if data["negativeMeaning"] == nil || data["name"] == nil {
							continue
						}

						obj := CriterionStruct{Name: data["name"].(string), NegativeMeaning: data["negativeMeaning"].(string)}

						if _, ok := negMeanings[obj]; ok {
							negMeanings[obj] = attr.Value
							//fmt.Printf("found negMeaning record %s for '%.190s'\n", attr.Value, data["negativeMeaning"].(string))
							_, err = temp.Write([]byte(attr.Value + "   " + obj.NegativeMeaning + " -- " + obj.Name + "\n"))
							if err != nil {
								log.Fatalf("writing temp_file failure: %v", err)
							}

							/*
								exit := false
								for !exit {
									token2, err := decoder.Token()
									if err != nil {
										break
									}
									switch t2 := token2.(type) {
									case xml.CharData:
										//fmt.Printf("attrs ---- %s\n", strings.TrimSpace(string(t2)))
										if str_t := strings.TrimSpace(string(t2)); str_t != "" {
											//fmt.Printf("attrs ---- %s\n", str_t)

											var data map[string]interface{}

											err := json.Unmarshal([]byte(str_t), &data)
											if err != nil {
												log.Fatalf("json parsing failure: %v", err)
											}

											rejections[attr.Value] = data["name"].(string)
											exit = true
										}
									}
								}
							*/
						}
					}
				}
			}
		}
	}

	/*n := 1
	for item := range negMeanings {
		fmt.Printf("%d. %d %s\n", n, len([]rune(item)), item)
		n++
	}*/

	text := `do
$$
declare
    rec record;
    key_type int8;
begin
    for rec in (select scheme from regadm.m_projects where scheme = 'kostgo') loop
        perform set_config('search_path', rec.scheme, true);
        raise info '%', rec.scheme;

        select key into key_type from kostgo.d_ref_dependent_type where full_name like 'DocRefRejectReasonType';
        raise info 'key of DocRefRejectReasonType = %', key_type;
`

	//fmt.Println(text)

	i := 0
	for rejectName, rejectId := range rejections {
		i++
		text += fmt.Sprintf("        -- %d characters in name\n        ", len([]rune(rejectName)))

		if rejectId == "TRASH" {
			text += "--"
		}
		text += fmt.Sprintf("insert into d_ref_dependents (alias, code_kcr, dependent_type, full_name, name, is_draft, sys_status) values ('Opal%sRejReason%d', '%s', key_type, '%s','%s', 0, 0);\n",
			service, i, rejectId, rejectName, rejectName)
	}
	text += "\n"
	for obj, negMeanId := range negMeanings {
		i++
		text += fmt.Sprintf("        -- %d characters in name\n        ", len([]rune(obj.NegativeMeaning)))

		if negMeanId == "TRASH" {
			text += fmt.Sprintf("--name in dictionary: %s\n        --", obj.Name)
		}
		text += fmt.Sprintf("insert into d_ref_dependents (alias, code_kcr, dependent_type, full_name, name, is_draft, sys_status) values ('Opal%sRejMeaning%d', '%s', key_type, '%s','%s', 0, 0);\n",
			service, i, negMeanId, obj.NegativeMeaning, obj.NegativeMeaning)
	}

	text += `    end loop;
end;
$$;`

	outputFile, err := os.Create("script_to_add_rejection_reasons.sql")
	if err != nil {
		log.Fatalf("output creation failure: %v", err)
	}
	defer outputFile.Close()
	_, err = outputFile.Write([]byte(text))
	if err != nil {
		log.Fatalf("writing output failure: %v", err)
	}
	//fmt.Println(text)

	//fmt.Println(len([]rune("Заключение, подтверждающие соблюдение требований технических регламентов при размещении планируемого к строительству или реконструкции объекта капитального строительства.pdf")))
	/*
	   	fmt.Println(len("наличие у уполномоченных на выдачу разрешений на строительство федерального органа исполнительной власти, органа исполнительной власти субъекта Российской Федерации,
	       органа местного самоуправления, государственной корпорации по атомной энергии Росатом или государственной корпорации по космической деятельности Роскосмос информации о выявленном в
	       рамках государственного строительного надзора, государственного земельного надзора или муниципального земельного контроля факте отсутствия начатых работ по строительству, реконструкции
	       на день подачи заявления о внесении изменений в разрешение на строительство в связи с продлением срока действия такого разрешения или информации органа государственного строительного
	       надзора об отсутствии извещения о начале данных работ, если направление такого извещения является обязательным в соответствии с требованиями части 5 статьи 52 Градостроительного кодекса Российской Федерации, в случае, если внесение изменений в разрешение на строительство связано с продлением срока действия разрешения на строительство"))
	*/
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

VS01187003GGER03_3T
["VS01187003GGER03_3T"]

18:30:35.393 INFO 100,00% Выполнение миграции данных
18:30:36.233 INFO 100,00% 00:00:00.8403659 Миграция данных - пропущено

log_path="$dir_path/$schema_name.log"
last_string_num=$(wc -l $log_path | sed s!$log_path!!)
finish_line=$(sed -n "${last_string_num}p" $log_path)


Ошибка передачи элемента: "{"ID":"d9c77f45-f856-422f-bcf7-6214112656ff","Denomination":null,"State":null,"Type":null,"Arrangement":null,"Location":null,"CadastralNumber":null,"Len
 System.AggregateException: One or more errors occurred. (Ошибка отправки XML пакета: "{
  "errorId": "00-258b1a254d910b7cd132e8349d35c98a-2f607b2d7c3bf217-00",
  "message": "Some kind of error happened in the API. Errors: The element \u0027Fields\u0027 has incomplete content. List of possible elements expected: \u0027Source\u0027."
}")




trigger_word='FATAL'
log_path="ekbgo.log"
last_lines=$(tail -n 50 "$log_path")

if [[ "$last_lines" != *"$trigger_word"* ]];
then
    echo "Метаданные ЗАЛИТЫ"
else
    echo "НЕ УДАЛОСЬ залить метаданные"
fi



"0af40b04-7295-1661-8172-c1ee57272d0f",

"INTERAGENCY_REQUEST_SENDING": [
    "0ae94758-7be3-1c2b-817b-f23748c80027",
    "0ae96485-8abd-1f11-818b-8b789ad0543a",
    "0ae947aa-8632-122f-8186-4a08412904aa",
    "0ae95269-921d-1155-8192-1f4472220142",
    "a50b8a3c-b546-11ea-b804-03b6cb4f5441",
    "a50b6336-b546-11ea-b803-2394ab708be7",
        "0ae95273-915a-1ad8-8191-7eed50020602",
        "0ae947aa-8632-122f-8186-4a08412a04ab",
        "0ae9644c-87a3-197a-8187-b2eb1dfb00f6",
        "0ae947f5-89b6-15bc-8189-ba5cf8ce0064",
        "0ae9526d-8abd-1e6c-818b-ccd2bdf84acb",
        "0ae95269-921d-1155-8192-9b2707b71c7a",
        "0ae9645e-9027-1bf5-8190-3a3c139b04cd",
        "0ae95269-921d-1155-8192-1f4472220142"

  ],
*/
