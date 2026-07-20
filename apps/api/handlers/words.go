package handlers

// vocab is one curriculum entry: an essential Italian word and the English
// word with the same meaning. Both uppercase A–Z; lengths vary freely.
type vocab struct {
	Italian string
	English string
}

// words is the beginner curriculum, in teaching order: cognates (Italian words
// an English speaker can almost guess) come first, then everyday themed
// vocabulary. Accented words (caffè, città, …) are excluded so the game
// sticks to plain A–Z.
var words = []vocab{
	// --- cognates & near-cognates: the gentlest on-ramp ---
	{"TRENO", "TRAIN"},
	{"BANCA", "BANK"},
	{"MUSICA", "MUSIC"},
	{"TEATRO", "THEATER"},
	{"LIMONE", "LEMON"},
	{"MELONE", "MELON"},
	{"LEONE", "LION"},
	{"TIGRE", "TIGER"},
	{"ZEBRA", "ZEBRA"},
	{"ELEFANTE", "ELEPHANT"},
	{"DELFINO", "DOLPHIN"},
	{"GIRAFFA", "GIRAFFE"},
	{"STAZIONE", "STATION"},
	{"OSPEDALE", "HOSPITAL"},
	{"RISTORANTE", "RESTAURANT"},
	{"AEROPORTO", "AIRPORT"},
	{"BICICLETTA", "BICYCLE"},
	{"TELEFONO", "TELEPHONE"},
	{"MOMENTO", "MOMENT"},
	{"MINUTO", "MINUTE"},
	{"PROBLEMA", "PROBLEM"},
	{"PERSONA", "PERSON"},
	{"ANIMALE", "ANIMAL"},
	{"COLORE", "COLOR"},
	{"NUMERO", "NUMBER"},
	{"CENTRO", "CENTER"},
	{"MUSEO", "MUSEUM"},
	{"PARCO", "PARK"},
	{"ISOLA", "ISLAND"},
	{"COSTA", "COAST"},
	{"PORTO", "PORT"},
	{"ARTE", "ART"},
	{"STUDENTE", "STUDENT"},
	{"DOTTORE", "DOCTOR"},
	{"FAMOSO", "FAMOUS"},
	{"FAMIGLIA", "FAMILY"},
	{"LAMPADA", "LAMP"},
	{"LETTERA", "LETTER"},
	{"FORESTA", "FOREST"},
	{"FRUTTA", "FRUIT"},
	{"INSALATA", "SALAD"},
	{"POMODORO", "TOMATO"},
	{"BANANA", "BANANA"},
	{"CIOCCOLATO", "CHOCOLATE"},
	{"MONTAGNA", "MOUNTAIN"},
	{"GIARDINO", "GARDEN"},

	// --- food & drink ---
	{"ACQUA", "WATER"},
	{"PANE", "BREAD"},
	{"LATTE", "MILK"},
	{"VINO", "WINE"},
	{"BIRRA", "BEER"},
	{"FORMAGGIO", "CHEESE"},
	{"UOVO", "EGG"},
	{"MELA", "APPLE"},
	{"PESCE", "FISH"},
	{"CARNE", "MEAT"},
	{"POLLO", "CHICKEN"},
	{"RISO", "RICE"},
	{"SALE", "SALT"},
	{"ZUCCHERO", "SUGAR"},
	{"BURRO", "BUTTER"},

	// --- animals ---
	{"GATTO", "CAT"},
	{"CANE", "DOG"},
	{"CAVALLO", "HORSE"},
	{"UCCELLO", "BIRD"},
	{"MUCCA", "COW"},

	// --- nature & time ---
	{"SOLE", "SUN"},
	{"LUNA", "MOON"},
	{"STELLA", "STAR"},
	{"MARE", "SEA"},
	{"FIUME", "RIVER"},
	{"ALBERO", "TREE"},
	{"FIORE", "FLOWER"},
	{"CIELO", "SKY"},
	{"FUOCO", "FIRE"},
	{"TERRA", "EARTH"},
	{"VENTO", "WIND"},
	{"PIOGGIA", "RAIN"},
	{"NEVE", "SNOW"},
	{"NOTTE", "NIGHT"},
	{"GIORNO", "DAY"},
	{"TEMPO", "TIME"},
	{"ANNO", "YEAR"},
	{"MESE", "MONTH"},
	{"SETTIMANA", "WEEK"},
	{"OGGI", "TODAY"},
	{"DOMANI", "TOMORROW"},

	// --- people & family ---
	{"UOMO", "MAN"},
	{"DONNA", "WOMAN"},
	{"BAMBINO", "CHILD"},
	{"AMICO", "FRIEND"},
	{"MADRE", "MOTHER"},
	{"PADRE", "FATHER"},
	{"FRATELLO", "BROTHER"},
	{"SORELLA", "SISTER"},

	// --- body ---
	{"TESTA", "HEAD"},
	{"OCCHIO", "EYE"},
	{"NASO", "NOSE"},
	{"BOCCA", "MOUTH"},
	{"MANO", "HAND"},
	{"PIEDE", "FOOT"},
	{"CUORE", "HEART"},

	// --- home & everyday things ---
	{"LIBRO", "BOOK"},
	{"PORTA", "DOOR"},
	{"FINESTRA", "WINDOW"},
	{"TAVOLO", "TABLE"},
	{"SEDIA", "CHAIR"},
	{"LETTO", "BED"},
	{"CUCINA", "KITCHEN"},
	{"BAGNO", "BATHROOM"},
	{"CHIAVE", "KEY"},
	{"SOLDI", "MONEY"},
	{"LAVORO", "WORK"},
	{"SCUOLA", "SCHOOL"},
	{"STRADA", "STREET"},
	{"MACCHINA", "CAR"},
	{"NAVE", "SHIP"},
	{"PONTE", "BRIDGE"},

	// --- colors & qualities ---
	{"ROSSO", "RED"},
	{"VERDE", "GREEN"},
	{"GIALLO", "YELLOW"},
	{"NERO", "BLACK"},
	{"BIANCO", "WHITE"},
	{"AZZURRO", "BLUE"},
	{"GRANDE", "BIG"},
	{"PICCOLO", "SMALL"},
	{"NUOVO", "NEW"},
	{"VECCHIO", "OLD"},
	{"CALDO", "HOT"},
	{"FREDDO", "COLD"},
	{"FELICE", "HAPPY"},
	{"TRISTE", "SAD"},
	{"VELOCE", "FAST"},
	{"LENTO", "SLOW"},
	{"FACILE", "EASY"},

	// --- abstract essentials ---
	{"AMORE", "LOVE"},
	{"PACE", "PEACE"},
	{"VITA", "LIFE"},
	{"MONDO", "WORLD"},
	{"PAESE", "COUNTRY"},
	{"LINGUA", "LANGUAGE"},
	{"PAROLA", "WORD"},
}

// english maps an Italian curriculum word to its translation.
var english = func() map[string]string {
	m := make(map[string]string, len(words))
	for _, v := range words {
		m[v.Italian] = v.English
	}
	return m
}()
