package audit

// Labels and search aliases have one source, shared by SQL and public UI metadata.
type vocabulary struct {
	Label   string `json:"label"`
	Aliases string `json:"aliases"`
}

var actionCatalog = map[string]vocabulary{
	"corrected":                  {"Исправление закрытой отметки", "исправлена исправлено исправлен"},
	"journal_opened":             {"Открытие журнала", "открыт открыто"},
	"journal_closed":             {"Завершение журнала", "завершен закрыт"},
	"marked":                     {"Отметка посещаемости", "отмечена отмечен"},
	"created":                    {"Создание", "создан создана создано"},
	"updated":                    {"Изменение", "изменен изменена обновлен обновлена"},
	"archived":                   {"Архивирование", "архив архивирован"},
	"restored":                   {"Возврат из архива", "восстановлен восстановлена"},
	"administrator_created":      {"Создание администратора", "создан создана"},
	"administration_updated":     {"Изменение администратора", "изменен изменена"},
	"trainer_created":            {"Создание тренера", "создан создана"},
	"trainer_account_updated":    {"Изменение аккаунта тренера", "изменен изменена"},
	"trainer_password_changed":   {"Смена пароля тренера", "изменен"},
	"trainer_assignment_created": {"Назначение тренера", "назначен назначена"},
	"trainer_assignment_revoked": {"Отзыв назначения тренера", "отозван"},
	"added":                      {"Включение в группу", "добавлен включен"},
	"excluded":                   {"Исключение из группы", "исключен"},
	"transferred":                {"Перевод", "переведен переведена"},
	"ended":                      {"Завершение периода", "завершен"},
	"superseded":                 {"Замена планового занятия", "заменено"},
	"cancelled":                  {"Отмена занятия", "отменено отменен"},
	"cancel":                     {"Отмена занятия", "отменено отменен"},
	"replace-trainer":            {"Замена тренера", "заменен заменена"},
	"trainer_replaced":           {"Замена тренера", "заменен заменена"},
	"reschedule":                 {"Перенос занятия", "перенесено перенесен"},
}

var entityCatalog = map[string]vocabulary{
	"attendance_record": {"Отметка посещаемости", "посещение"},
	"club":              {"Филиал", "клуб"},
	"staff_user":        {"Сотрудник", "аккаунт"},
	"club_membership":   {"Карточка участника", "спортсмен"},
	"training_group":    {"Группа", ""},
	"group_membership":  {"Период в группе", "состав"},
	"schedule_rule":     {"Правило расписания", ""},
	"training_session":  {"Занятие", "тренировка"},
}

func labels(catalog map[string]vocabulary) map[string]string {
	result := make(map[string]string, len(catalog))
	for code, value := range catalog {
		result[code] = value.Label
	}
	return result
}

type Actor struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}
