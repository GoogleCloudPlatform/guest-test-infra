package imagevalidation

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var licenseNames = []string{
	"Apache License",
	"Artistic/GPL",
	"Artistic",
	"Autoconf",
	"\"Bitstream Vera\"",
	"BSD",
	"BZIP",
	"COMMON PUBLIC LICENSE VERSION 1.0",
	"config-h",
	"curl",
	"Expat",
	"GAP",
	"GD",
	"GFDL-1.3+",
	"GNU General Public License",
	"GNU GPL",
	"GNU LGPL",
	"GNU Lesser Public License",
	"GPL",
	"HPND",
	"IBM PUBLIC LICENSE VERSION 1.0",
	"ISC",
	"JPEG",
	"LGPL",
	"MIT",
	"MIT license",
	"MIT/X11 (BSD like)",
	"no notice",
	"noderivs",
	"none",
	"PD",
	"PD-debian",
	"PERLDOCS",
	"permissive-fsf",
	"permissive-nowarranty",
	"probably-PD",
	"Paul Vixie\"s license",
	"public-domain",
	"REGCOMP",
	"S2P",
	"SDBM-PUBLIC-DOMAIN",
	"TEXT-SOUNDEX",
	"TEXT-TABS",
	"The OpenLDAP Public License",
	"Unicode",
	"X11",
	"X11-2",
	"ZLIB",
}

var licenses = []string{
	`Permission to use, copy, modify, distribute, and sell this software and its documentation for any purpose is hereby granted without fee, provided that the above copyright notice appear in all copies and that both that copyright notice and this permission notice appear in supporting documentation, and that the name of the authors not be used in advertising or publicity pertaining to distribution of the software without specific, written prior permission. The authors makes no representations about the suitability of this software for any purpose. It is provided "as is" without express or implied warranty.`,
	`free software; you can redistribute it and/or modify it under the terms of the GNU.*General Public License.*as published by the Free Software Foundation`,
	`The main library is licensed under GNU Lesser General Public License (LGPL) version 2.1+, Gnutls Extra (i.e. openssl wrapper library, and library for code for "GnuTLS Inner Application" support) build system, testsuite and commandline utilities are licenced under the GNU General Public License version 3+. The Guile bindings use the same license as the respective underlying library, i.e. LGPLv2.1+ for the main library and GPLv3+ for Gnutls extra.`,
	`Permission is granted to anyone to use this.*for any purpose, including commercial applications, and to alter it and redistribute it freely, subject to the following restrictions`,
	`This software is released under the terms of the GNU.*General Public License.*`,
	`All files in this package can be freely distributed and used according to the terms of the GNU.*General Public License, either version 2 or (at your opinion) any newer version. This is the same distribution policy as for the Linux kernel itself -- see /usr/src/linux/COPYING for details.`,
	`You are free to distribute this.*under the terms of the BSD License`,
	`All files in this.*can be freely distributed and used according to the terms of the GNU General Public License`,
	`all of the code is covered under the terms of the GPL.`,
	`is free software`,
	`You are free to distribute this software under the terms of the BSD License.`,
	`is licensed under the BSD license`,
	`(is|are|be) free to distribute`,
	`may freely distribute`,
	`(is|are|be) freely distributed`,
	`.*is available under the terms of the GNU.*Public License`,
	`This data is licenced under 2 different licenses 1\) GNU General Public License, version 2 or later 2\) XFree86 1.0 license This data can be used freely under either license.`,
	`.*is in the public domain.`,
	`is covered under the terms of the GNU Public License.`,
	`redistribute it freely`,
	`the complete text of the GNU General Public License can be found in`,
	`free for commercial and non-commercial use as long as the following conditions are aheared to`,
	`Permission to.*use.*distribute.*this.*for any purpose.*is.*granted`,
	`There are no restrictions on distributing unmodified copies of Vim except that they must include this license text.`,
	`Redistribution and use.*(is|are) permitted`,
	`Permission is.*granted.*deal.*without restriction, including without limitation the rights to use`,
	`All its programs.*may be redistributed under the terms of the GNU GPL, Version 2 or later`,
	`is distributed under the GNU.*General Public License`,
	`This software is distributed under the GNU General Public License`,
	`This package is dual-licensed under the Academic Free License version 2.1, and the GPL version 2.`,
	`may be used, modified and redistributed only under the terms of the GNU General Public License`,
	`has been placed in the public domain`,
	`And licensed under the terms of the GPL license`,
	`are distributed under the terms of the GNU.*General Public License`,
	`The keys in the keyrings don\'t fall under any copyright. Everything else in the package is covered by the GNU GPL.`,
	`the complete text of the GNU General Public License and of the GNU Lesser Public License can be found in`,
	`THE ACCOMPANYING PROGRAM IS PROVIDED UNDER THE TERMS OF THIS IBM PUBLIC LICENSE`,
	`THE ACCOMPANYING PROGRAM IS PROVIDED UNDER THE TERMS OF THIS COMMON PUBLIC LICENSE`,
	`GNU LESSER GENERAL PUBLIC LICENSE`,
	`Permission is hereby granted.*to any person obtaining a copy of.*and associated documentation files.*to deal in.*without restriction`,
	`Redistribution and use of this software and associated documentation ("Software"), with or without modification, are permitted`,
	`This code is multi Licensed under all/any one of.*LGPLv2.*New Style BSD.*MIT`,
	`LICENSE. You may copy and use the Software, subject to these conditions: 1. This Software is licensed for use only in conjunction with Intel component products. Use of the Software in conjunction with non-Intel component products is not licensed hereunder.`,
	`Brocade Linux Fibre Channel HBA Firmware`,
	`QLogic Linux Fibre Channel HBA Firmware`,
	`Unlimited distribution and/or modification is allowed as long as this copyright notice remains intact.`,
	`Permission is hereby granted to use.*this.*for any purpose`,
	`are in the public domain`,
	`is (available|distributed) under the terms of the GNU.*Public License`,
	`(libudev|libgudev|udev) is licensed under the GNU (L|)GPL`,
	`The Linux Console Tools are covered by the GPL`,
	`Some portions of os-prober`,
	`Netcat and the associated package is a product of Avian Research, and is freely available in full source form with no restrictions save an obligation to give credit where due.`,
	`Permission is hereby granted, without written agreement and without licence or royalty fees, to use, copy, modify, and distribute this software`,
	`Open Market permits you to use, copy, modify, distribute, and license this Software and the Documentation for any purpose, provided that existing copyright notices are retained in all copies and that this notice is included verbatim in any distributions. No written agreement, license, or royalty fee is required for any of the authorized uses.`,
	`This software is made available under the terms of *either* of the licenses found in LICENSE.APACHE or LICENSE.BSD. Contributions to cryptography are made under the terms of *both* these licenses.`,
}

const (
	copyright = "/usr/share/doc/*/copyright"
	license   = "/usr/share/doc/*/LICENSE"
)

func isValidLicenseName(licenseCheck string) bool {
	for _, name := range licenseNames {
		// (?i) case insensitive
		var regexString = fmt.Sprintf(`(?i)((?:(?:License|Copyright)\s*:\s*%[1]s)|(?:(?:covered )*under (?:the )?%[1]s)|(?:under (?:the terms of )*the %[1]s))`, name)
		re := regexp.MustCompile(regexString)

		if re.MatchString(licenseCheck) {
			return true
		}
	}
	return false
}

func isValidLicenseText(licenseCheck string) bool {
	for _, licenseText := range licenses {
		// (?i) case insensitive
		re := regexp.MustCompile(`(?i)(` + licenseText + `)`)

		if re.MatchString(licenseCheck) {
			return true
		}
	}
	return false
}

func isValidLicense(licenseCheck string) bool {
	return isValidLicenseName(licenseCheck) || isValidLicenseText(licenseCheck)
}

func TestArePackagesLegal(t *testing.T) {
	var filenames []string
	if utils.IsTargetLinuxVersion(utils.RedHat) || utils.IsTargetLinuxVersion(utils.SUSE) {
		filenames, _ = filepath.Glob(license)
	} else if utils.IsTargetLinuxVersion(utils.Debian) || utils.IsTargetLinuxVersion(utils.Ubuntu) {
		filenames, _ = filepath.Glob(copyright)
	} else {
		t.Skip("can not run test on other os")
	}
	for _, filename := range filenames {
		if !isPackageLegal(filename) {
			t.Fatalf("The packages are not legal to use")
		}
	}
}

func isPackageLegal(filepath string) bool {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Printf("error read file")
	}

	var licenseCheck string = string(bytes)

	// Remove comment
	re := regexp.MustCompile(`(\*|#)*`)
	loc := re.FindStringIndex(licenseCheck)
	if loc != nil {
		licenseCheck = licenseCheck[0:loc[0]] + licenseCheck[loc[1]:]
	}
	// Replace all whitespace with one space
	licenseCheck = strings.Join(strings.Fields(licenseCheck), " ")
	if !isValidLicense(licenseCheck) {
		fmt.Printf("The package %s are not legal to use.", filepath)
		return false
	}
	return true
}
