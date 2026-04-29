package httpx

import (
	"net/http"
	"strconv"

	"github.com/mistypass/cloud/api/internal/modules/access"
)

// --- Holiday Calendar Preset Countries and Entries ---

type holidayPresetCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var presetCountries = []holidayPresetCountry{
	{Code: "ID", Name: "Indonesia"},
	{Code: "US", Name: "United States"},
	{Code: "CN", Name: "China"},
	{Code: "SG", Name: "Singapore"},
	{Code: "MY", Name: "Malaysia"},
}

func (s *server) listHolidayCalendarPresetCountries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": presetCountries,
	})
}

func (s *server) listHolidayCalendarPresets(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	if country == "" {
		writeError(w, http.StatusBadRequest, "country query parameter is required")
		return
	}
	yearStr := r.URL.Query().Get("year")
	year := 2026
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y >= 2024 && y <= 2030 {
			year = y
		}
	}
	entries, countryName := holidayPresetEntries(country, year)
	if entries == nil {
		writeError(w, http.StatusNotFound, "no presets available for country: "+country)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"country":      country,
		"country_name": countryName,
		"year":         year,
		"entries":      entries,
	})
}

func holidayPresetEntries(country string, year int) ([]access.HolidayEntry, string) {
	y := strconv.Itoa(year)
	switch country {
	case "ID":
		return indonesiaHolidays(y), "Indonesia"
	case "US":
		return usHolidays(y), "United States"
	case "CN":
		return chinaHolidays(y), "China"
	case "SG":
		return singaporeHolidays(y), "Singapore"
	case "MY":
		return malaysiaHolidays(y), "Malaysia"
	default:
		return nil, ""
	}
}

func indonesiaHolidays(y string) []access.HolidayEntry {
	return []access.HolidayEntry{
		{Date: y + "-01-01", Name: "Tahun Baru Masehi", Description: "New Year's Day"},
		{Date: y + "-02-01", Name: "Tahun Baru Imlek", Description: "Chinese New Year"},
		{Date: y + "-03-12", Name: "Hari Raya Nyepi", Description: "Balinese Day of Silence"},
		{Date: y + "-03-20", Name: "Maulid Nabi Muhammad", Description: "Birthday of Prophet Muhammad"},
		{Date: y + "-03-28", Name: "Hari Raya Idul Fitri", Description: "Eid al-Fitr Day 1"},
		{Date: y + "-03-29", Name: "Hari Raya Idul Fitri", Description: "Eid al-Fitr Day 2"},
		{Date: y + "-04-02", Name: "Wafat Isa Al-Masih", Description: "Good Friday"},
		{Date: y + "-05-01", Name: "Hari Buruh", Description: "Labour Day"},
		{Date: y + "-05-12", Name: "Hari Raya Waisak", Description: "Vesak Day"},
		{Date: y + "-05-14", Name: "Kenaikan Isa Al-Masih", Description: "Ascension of Jesus Christ"},
		{Date: y + "-06-01", Name: "Hari Lahir Pancasila", Description: "Pancasila Day"},
		{Date: y + "-06-04", Name: "Hari Raya Idul Adha", Description: "Eid al-Adha"},
		{Date: y + "-06-25", Name: "Tahun Baru Islam", Description: "Islamic New Year"},
		{Date: y + "-08-17", Name: "Hari Kemerdekaan RI", Description: "Independence Day"},
		{Date: y + "-09-04", Name: "Maulid Nabi Muhammad", Description: "Birthday of Prophet Muhammad (observed)"},
		{Date: y + "-12-25", Name: "Hari Natal", Description: "Christmas Day"},
	}
}

func usHolidays(y string) []access.HolidayEntry {
	return []access.HolidayEntry{
		{Date: y + "-01-01", Name: "New Year's Day"},
		{Date: y + "-01-20", Name: "Martin Luther King Jr. Day"},
		{Date: y + "-02-17", Name: "Presidents' Day"},
		{Date: y + "-05-26", Name: "Memorial Day"},
		{Date: y + "-06-19", Name: "Juneteenth"},
		{Date: y + "-07-04", Name: "Independence Day"},
		{Date: y + "-09-01", Name: "Labor Day"},
		{Date: y + "-10-13", Name: "Columbus Day"},
		{Date: y + "-11-11", Name: "Veterans Day"},
		{Date: y + "-11-27", Name: "Thanksgiving Day"},
		{Date: y + "-12-25", Name: "Christmas Day"},
	}
}

func chinaHolidays(y string) []access.HolidayEntry {
	return []access.HolidayEntry{
		{Date: y + "-01-01", Name: "元旦", Description: "New Year's Day"},
		{Date: y + "-01-28", Name: "春节", Description: "Chinese New Year Eve"},
		{Date: y + "-01-29", Name: "春节", Description: "Chinese New Year Day 1"},
		{Date: y + "-01-30", Name: "春节", Description: "Chinese New Year Day 2"},
		{Date: y + "-01-31", Name: "春节", Description: "Chinese New Year Day 3"},
		{Date: y + "-02-01", Name: "春节", Description: "Chinese New Year Day 4"},
		{Date: y + "-02-02", Name: "春节", Description: "Chinese New Year Day 5"},
		{Date: y + "-02-03", Name: "春节", Description: "Chinese New Year Day 6"},
		{Date: y + "-04-04", Name: "清明节", Description: "Qingming Festival"},
		{Date: y + "-05-01", Name: "劳动节", Description: "Labour Day"},
		{Date: y + "-05-31", Name: "端午节", Description: "Dragon Boat Festival"},
		{Date: y + "-10-01", Name: "国庆节", Description: "National Day"},
		{Date: y + "-10-02", Name: "国庆节", Description: "National Day Holiday"},
		{Date: y + "-10-03", Name: "国庆节", Description: "National Day Holiday"},
		{Date: y + "-10-06", Name: "中秋节", Description: "Mid-Autumn Festival"},
	}
}

func singaporeHolidays(y string) []access.HolidayEntry {
	return []access.HolidayEntry{
		{Date: y + "-01-01", Name: "New Year's Day"},
		{Date: y + "-01-29", Name: "Chinese New Year Day 1"},
		{Date: y + "-01-30", Name: "Chinese New Year Day 2"},
		{Date: y + "-03-28", Name: "Hari Raya Puasa", Description: "Eid al-Fitr"},
		{Date: y + "-04-02", Name: "Good Friday"},
		{Date: y + "-05-01", Name: "Labour Day"},
		{Date: y + "-05-12", Name: "Vesak Day"},
		{Date: y + "-06-04", Name: "Hari Raya Haji", Description: "Eid al-Adha"},
		{Date: y + "-08-09", Name: "National Day"},
		{Date: y + "-10-20", Name: "Deepavali"},
		{Date: y + "-12-25", Name: "Christmas Day"},
	}
}

func malaysiaHolidays(y string) []access.HolidayEntry {
	return []access.HolidayEntry{
		{Date: y + "-01-01", Name: "New Year's Day"},
		{Date: y + "-01-29", Name: "Chinese New Year Day 1"},
		{Date: y + "-01-30", Name: "Chinese New Year Day 2"},
		{Date: y + "-02-01", Name: "Thaipusam"},
		{Date: y + "-03-28", Name: "Hari Raya Aidilfitri", Description: "Eid al-Fitr Day 1"},
		{Date: y + "-03-29", Name: "Hari Raya Aidilfitri", Description: "Eid al-Fitr Day 2"},
		{Date: y + "-05-01", Name: "Labour Day"},
		{Date: y + "-05-12", Name: "Wesak Day"},
		{Date: y + "-06-02", Name: "Yang di-Pertuan Agong Birthday"},
		{Date: y + "-06-04", Name: "Hari Raya Haji", Description: "Eid al-Adha"},
		{Date: y + "-06-25", Name: "Awal Muharram", Description: "Islamic New Year"},
		{Date: y + "-08-31", Name: "Merdeka Day", Description: "National Day"},
		{Date: y + "-09-04", Name: "Maulidur Rasul", Description: "Birthday of Prophet Muhammad"},
		{Date: y + "-09-16", Name: "Malaysia Day"},
		{Date: y + "-10-20", Name: "Deepavali"},
		{Date: y + "-12-25", Name: "Christmas Day"},
	}
}
