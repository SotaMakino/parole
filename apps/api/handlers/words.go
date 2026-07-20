package handlers

// Italian five-letter words. This list is both the answer pool (new games pick
// a random entry) and the accepted-guess dictionary. Accented words (caffè,
// città, …) are excluded so the game sticks to plain A–Z letters.
var words = []string{
	"ACETO", "ACQUA", "AGLIO", "AMARE", "AMICO", "AMORE", "ANIMA", "ASINO", "AVERE",
	"BACIO", "BAGNO", "BALLO", "BANCO", "BARCA", "BASSO", "BELLO", "BIRRA", "BOCCA",
	"BOSCO", "BRAVO", "BUONO", "BURRO",
	"CALDO", "CALMA", "CAMPO", "CANTO", "CAPRA", "CARNE", "CARTA", "CENTO", "CERVO",
	"CIELO", "COLLE", "COLPO", "CONTO", "CORPO", "CORSA", "CORTO", "CORVO", "CREMA",
	"CUOCO", "CUORE",
	"DAINO", "DENTE", "DIECI", "DOLCE", "DONNA", "DUOMO",
	"FALCO", "FARRO", "FESTA", "FIENO", "FIORE", "FIUME", "FORNO", "FORTE", "FORZA",
	"FUNGO", "FUOCO", "FURBO",
	"GALLO", "GATTO", "GENTE", "GESSO", "GIOCO", "GIOIA", "GONNA", "GRANO", "GUSTO",
	"LADRO", "LAMPO", "LARGO", "LATTE", "LEGNO", "LENTO", "LEONE", "LEPRE", "LETTO",
	"LIBRO", "LITRO", "LUOGO", "LUNGO",
	"MAGRO", "MARZO", "MENTE", "MERLO", "METRO", "MEZZO", "MIELE", "MILLE", "MONDO",
	"MONTE", "MORTE", "MOSCA", "MOSSA", "MUCCA",
	"NERVO", "NONNA", "NONNO", "NOTTE", "NUOVO",
	"OLIVA", "OMBRA",
	"PALCO", "PALLA", "PANCA", "PANNA", "PANNO", "PARCO", "PASTA", "PASTO", "PATTO",
	"PAURA", "PELLE", "PESCA", "PESCE", "PETTO", "PIANO", "PIEDE", "PIENO", "PIZZA",
	"POLLO", "PONTE", "PORTA", "PORTO", "POSTO", "PRATO", "PRIMO", "PUNTO",
	"QUOTA",
	"RAGNO", "RATTO", "REGNO", "RESTO", "RICCO", "ROSSO", "RUOTA",
	"SALSA", "SALTO", "SANTO", "SASSO", "SCALA", "SCENA", "SCOPO", "SEDIA", "SEGNO",
	"SENSO", "SETTE", "SFIDA", "SOGNO", "SOLDO", "SONNO", "SPESA", "STARE", "STILE",
	"SUCCO", "SUONO",
	"TARDI", "TEMPO", "TERRA", "TESTA", "TETTO", "TIGRE", "TONNO", "TORRE", "TORTA",
	"TRENO", "TROTA", "TUTTO",
	"UDIRE", "USARE",
	"VENTI", "VENTO", "VERDE", "VESPA", "VETRO", "VIOLA", "VISTA", "VOLPE", "VOLTA",
	"VOLTO", "VUOTO",
	"ZAINO", "ZEBRA", "ZUCCA",
}

var wordSet = func() map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}()
