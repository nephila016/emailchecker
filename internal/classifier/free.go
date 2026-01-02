package classifier

import (
	"strings"
)

// Free email provider domains
var freeProviders = map[string]bool{
	// Google
	"gmail.com":         true,
	"googlemail.com":    true,

	// Microsoft
	"outlook.com":       true,
	"hotmail.com":       true,
	"hotmail.co.uk":     true,
	"hotmail.fr":        true,
	"hotmail.de":        true,
	"hotmail.it":        true,
	"hotmail.es":        true,
	"live.com":          true,
	"live.co.uk":        true,
	"live.fr":           true,
	"live.de":           true,
	"msn.com":           true,

	// Yahoo
	"yahoo.com":         true,
	"yahoo.co.uk":       true,
	"yahoo.fr":          true,
	"yahoo.de":          true,
	"yahoo.it":          true,
	"yahoo.es":          true,
	"yahoo.co.in":       true,
	"yahoo.ca":          true,
	"yahoo.com.au":      true,
	"yahoo.com.br":      true,
	"yahoo.co.jp":       true,
	"ymail.com":         true,
	"rocketmail.com":    true,

	// AOL/Verizon
	"aol.com":           true,
	"aol.co.uk":         true,
	"aim.com":           true,
	"verizon.net":       true,

	// Apple
	"icloud.com":        true,
	"me.com":            true,
	"mac.com":           true,

	// ProtonMail
	"protonmail.com":    true,
	"protonmail.ch":     true,
	"proton.me":         true,
	"pm.me":             true,

	// Zoho
	"zoho.com":          true,
	"zohomail.com":      true,

	// Mail.com
	"mail.com":          true,
	"email.com":         true,
	"usa.com":           true,
	"post.com":          true,
	"europe.com":        true,
	"asia.com":          true,
	"consultant.com":    true,
	"engineer.com":      true,
	"doctor.com":        true,
	"lawyer.com":        true,
	"activist.com":      true,
	"accountant.com":    true,
	"techie.com":        true,
	"cheerful.com":      true,

	// GMX
	"gmx.com":           true,
	"gmx.net":           true,
	"gmx.de":            true,
	"gmx.at":            true,
	"gmx.ch":            true,

	// Yandex
	"yandex.com":        true,
	"yandex.ru":         true,
	"yandex.ua":         true,
	"ya.ru":             true,

	// Mail.ru
	"mail.ru":           true,
	"inbox.ru":          true,
	"bk.ru":             true,
	"list.ru":           true,

	// QQ/163
	"qq.com":            true,
	"163.com":           true,
	"126.com":           true,
	"sina.com":          true,
	"sina.cn":           true,
	"sohu.com":          true,
	"aliyun.com":        true,
	"foxmail.com":       true,

	// Tutanota
	"tutanota.com":      true,
	"tutanota.de":       true,
	"tutamail.com":      true,
	"tuta.io":           true,

	// FastMail
	"fastmail.com":      true,
	"fastmail.fm":       true,

	// Rediffmail
	"rediffmail.com":    true,
	"rediff.com":        true,

	// Regional/Country specific
	"web.de":            true,
	"freenet.de":        true,
	"t-online.de":       true,
	"libero.it":         true,
	"virgilio.it":       true,
	"free.fr":           true,
	"orange.fr":         true,
	"laposte.net":       true,
	"sfr.fr":            true,
	"wanadoo.fr":        true,
	"wp.pl":             true,
	"o2.pl":             true,
	"interia.pl":        true,
	"onet.pl":           true,
	"seznam.cz":         true,
	"centrum.cz":        true,
	"rambler.ru":        true,
	"ukr.net":           true,
	"i.ua":              true,
	"bigmir.net":        true,
	"naver.com":         true,
	"daum.net":          true,
	"hanmail.net":       true,
	"cox.net":           true,
	"att.net":           true,
	"sbcglobal.net":     true,
	"bellsouth.net":     true,
	"comcast.net":       true,
	"charter.net":       true,
	"earthlink.net":     true,
	"juno.com":          true,
	"optonline.net":     true,
	"shaw.ca":           true,
	"rogers.com":        true,
	"sympatico.ca":      true,
	"telus.net":         true,
	"btinternet.com":    true,
	"ntlworld.com":      true,
	"sky.com":           true,
	"blueyonder.co.uk":  true,
	"talktalk.net":      true,
	"virginmedia.com":   true,
	"bigpond.com":       true,
	"optusnet.com.au":   true,
	"ozemail.com.au":    true,

	// Indian providers
	"sify.com":          true,
	"indiatimes.com":    true,
	"sancharnet.in":     true,
	"dataone.in":        true,

	// Misc
	"lycos.com":         true,
	"excite.com":        true,
	"netscape.net":      true,
	"inbox.com":         true,
	"hushmail.com":      true,
	"runbox.com":        true,
	"lavabit.com":       true,
	"mailfence.com":     true,
	"disroot.org":       true,
	"riseup.net":        true,
	"autistici.org":     true,
	"inventati.org":     true,
}

// IsFreeProvider checks if domain is a free email provider
func IsFreeProvider(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return freeProviders[domain]
}

// GetFreeProviderCount returns the number of free providers in the list
func GetFreeProviderCount() int {
	return len(freeProviders)
}
