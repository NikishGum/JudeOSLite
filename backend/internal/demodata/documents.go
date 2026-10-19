package demodata

import (
	"encoding/json"
	"time"
)

// Some cards intentionally have no files, so empty states remain inspectable.
// All samples are explicitly labelled and have no private or medical content.
func (l *loader) documents(child, c int, anchor time.Time) {
	for kind, document := range []struct{ kind, name, filename string }{
		{"medical", "Медицинский допуск · учебный образец", "Учебный медицинский допуск.pdf"},
		{"contract", "Договор занятий · учебный образец", "Учебный договор.pdf"},
		{"consent", "Согласие родителя · учебный образец", "Учебное согласие.pdf"},
		{"other", "Памятка участнику · учебный образец", "Памятка участнику.pdf"},
	} {
		var expiry any
		if document.kind == "medical" {
			expiry = day(anchor.AddDate(0, 0, []int{-7, 14, 180}[child%3]))
		}
		did := id("document", child*4+kind)
		cid := id("club", c)
		mid := id("member", child)
		l.exec(`INSERT INTO pride.member_documents(id,club_id,membership_id,name,kind,filename,media_type,content,expires_on,created_by,created_at) VALUES($1,$2,$3,$4,$5,$6,'application/pdf',$7,$8::date,$9,$10)`, did, cid, mid, document.name, document.kind, document.filename, samplePDF(), expiry, id("admin", c*2), anchor.AddDate(0, 0, -10))
		state, _ := json.Marshal(map[string]any{"id": did, "membershipId": mid, "version": 1})
		l.exec(`INSERT INTO pride.audit_events(club_id,actor_id,entity_type,entity_id,action,after_state) VALUES($1,$2,'member_document',$3,'member_document_uploaded',$4)`, cid, id("admin", c*2), did, state)
	}
}
