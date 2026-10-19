package demodata

import "strings"

type givenName struct{ name, malePatronymic, femalePatronymic, latin string }

var maleNames = []givenName{
	{"Александр", "Александрович", "Александровна", "aleksandr"},
	{"Андрей", "Андреевич", "Андреевна", "andrey"},
	{"Дмитрий", "Дмитриевич", "Дмитриевна", "dmitriy"},
	{"Сергей", "Сергеевич", "Сергеевна", "sergey"},
	{"Алексей", "Алексеевич", "Алексеевна", "aleksey"},
	{"Иван", "Иванович", "Ивановна", "ivan"},
	{"Максим", "Максимович", "Максимовна", "maksim"},
	{"Михаил", "Михайлович", "Михайловна", "mikhail"},
	{"Павел", "Павлович", "Павловна", "pavel"},
	{"Виктор", "Викторович", "Викторовна", "viktor"},
	{"Роман", "Романович", "Романовна", "roman"},
	{"Николай", "Николаевич", "Николаевна", "nikolay"},
	{"Антон", "Антонович", "Антоновна", "anton"},
	{"Кирилл", "Кириллович", "Кирилловна", "kirill"},
	{"Артём", "Артёмович", "Артёмовна", "artem"},
}
var femaleNames = []struct{ name, latin string }{
	{"Анна", "anna"}, {"Елена", "elena"}, {"Наталья", "natalya"}, {"Ольга", "olga"},
	{"Татьяна", "tatyana"}, {"Марина", "marina"}, {"Ирина", "irina"}, {"Светлана", "svetlana"},
	{"Юлия", "yuliya"}, {"Екатерина", "ekaterina"}, {"Анастасия", "anastasiya"},
	{"Виктория", "viktoriya"}, {"Дарья", "darya"}, {"Полина", "polina"}, {"Софья", "sofya"},
}

// Explicit inflections keep family names and women's surnames natural.
var surnames = strings.Fields(`
Ковалёв|Ковалёва|Ковалёвых|kovalev Иванов|Иванова|Ивановых|ivanov
Новик|Новик|Новиков|novik Жук|Жук|Жуков|zhuk
Козлов|Козлова|Козловых|kozlov Мороз|Мороз|Морозов|moroz
Волков|Волкова|Волковых|volkov Соколов|Соколова|Соколовых|sokolov
Петров|Петрова|Петровых|petrov Васильев|Васильева|Васильевых|vasilyev
Мельник|Мельник|Мельников|melnik Романюк|Романюк|Романюков|romanyuk
Павлов|Павлова|Павловых|pavlov Григорьев|Григорьева|Григорьевых|grigoryev
Бондарь|Бондарь|Бондарей|bondar Савицкий|Савицкая|Савицких|savitskiy
Лис|Лис|Лисов|lis Кравцов|Кравцова|Кравцовых|kravtsov
Дрозд|Дрозд|Дроздов|drozd Ткачёв|Ткачёва|Ткачёвых|tkachev
Гончар|Гончар|Гончаров|gonchar Марков|Маркова|Марковых|markov
Карпов|Карпова|Карповых|karpov Семёнов|Семёнова|Семёновых|semenov
Зайцев|Зайцева|Зайцевых|zaytsev Фёдоров|Фёдорова|Фёдоровых|fedorov
Белов|Белова|Беловых|belov Орлов|Орлова|Орловых|orlov
Макаров|Макарова|Макаровых|makarov Киселёв|Киселёва|Киселёвых|kiselev
Захаров|Захарова|Захаровых|zakharov Климов|Климова|Климовых|klimov
Якубович|Якубович|Якубовичей|yakubovich Леонов|Леонова|Леоновых|leonov
Руденко|Руденко|Руденко|rudenko Котов|Котова|Котовых|kotov
Сидоров|Сидорова|Сидоровых|sidorov Кузнецов|Кузнецова|Кузнецовых|kuznetsov
Егоров|Егорова|Егоровых|egorov Денисов|Денисова|Денисовых|denisov
Баранов|Баранова|Барановых|baranov Антонов|Антонова|Антоновых|antonov
Голуб|Голуб|Голубей|golub Наумов|Наумова|Наумовых|naumov
Демидов|Демидова|Демидовых|demidov Лаптев|Лаптева|Лаптевых|laptev
Быков|Быкова|Быковых|bykov Мартынов|Мартынова|Мартыновых|martynov
Тарасов|Тарасова|Тарасовых|tarasov Осипов|Осипова|Осиповых|osipov
`)

type person struct{ name, login string }

func adult(index int, female bool, role string) person {
	s := strings.Split(surnames[index%len(surnames)], "|")
	first := fatherNameIndex(index)
	grand := maleNames[(index+7)%len(maleNames)]
	if female {
		grand = maleNames[(index+11)%len(maleNames)]
		n := femaleNames[(first+7)%len(femaleNames)]
		return person{s[1] + " " + n.name + " " + grand.femalePatronymic, role + "." + n.latin + "." + s[3]}
	}
	n := maleNames[first]
	return person{s[0] + " " + n.name + " " + grand.malePatronymic, role + "." + n.latin + "." + s[3]}
}
func childName(family, sibling int) string {
	s := strings.Split(surnames[family%len(surnames)], "|")
	father := maleNames[fatherNameIndex(family)]
	first := (family + 3 + sibling*5) % len(maleNames)
	if (family+sibling)%3 == 0 {
		return s[1] + " " + femaleNames[first].name + " " + father.femalePatronymic
	}
	return s[0] + " " + maleNames[first].name + " " + father.malePatronymic
}

func fatherNameIndex(index int) int {
	return (index/len(surnames) + (index%len(surnames))*7) % len(maleNames)
}

// Staff have separate identities from the fictional parents.
var trainers = []person{
	{"Лукашевич Андрей Викторович", "trainer.andrey.lukashevich"},
	{"Борисевич Дмитрий Сергеевич", "trainer.dmitriy.borisevich"},
	{"Романовский Алексей Петрович", "trainer.aleksey.romanovskiy"},
	{"Сташкевич Сергей Иванович", "trainer.sergey.stashkevich"},
	{"Костюкевич Максим Павлович", "trainer.maksim.kostyukevich"},
	{"Адамович Иван Андреевич", "trainer.ivan.adamovich"},
	{"Станкевич Михаил Александрович", "trainer.mikhail.stankevich"},
	{"Гурский Павел Дмитриевич", "trainer.pavel.gurskiy"},
	{"Дубовик Роман Николаевич", "trainer.roman.dubovik"},
	{"Шевцов Виктор Михайлович", "trainer.viktor.shevtsov"},
	{"Сорокин Кирилл Сергеевич", "trainer.kirill.sorokin"},
	{"Левкович Антон Викторович", "trainer.anton.levkovich"},
	{"Савченко Артём Алексеевич", "trainer.artem.savchenko"},
	{"Тимофеев Николай Павлович", "trainer.nikolay.timofeev"},
	{"Прокопов Александр Романович", "trainer.aleksandr.prokopov"},
}
var administrators = []person{
	{"Вишневская Елена Андреевна", "administrator.elena.vishnevskaya"},
	{"Чернявская Наталья Сергеевна", "administrator.natalya.chernyavskaya"},
	{"Калиновская Ольга Михайловна", "administrator.olga.kalinovskaya"},
	{"Михалевич Ирина Владимировна", "administrator.irina.mikhalevich"},
	{"Литвиненко Татьяна Александровна", "administrator.tatyana.litvinenko"},
	{"Рожков Алексей Иванович", "administrator.aleksey.rozhkov"},
}
