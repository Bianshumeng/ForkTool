package service

func UpdateStatusPageURL(url string) string {
	return normalizeStatusPageURL(url)
}

func normalizeStatusPageURL(url string) string {
	return url
}
