package image_validation

// List of relevant paragraphs for determining open-source-ness
/*
LICENSE_NAMES = [
    'Apache License',
    'Artistic/GPL',
    'Artistic',
    'Autoconf',
    '"Bitstream Vera"',
    'BSD',
    'BZIP',
    'COMMON PUBLIC LICENSE VERSION 1.0',
    'config-h',
    'curl',
    'Expat',
    'GAP',
    'GD',
    'GFDL-1.3+',
    'GNU General Public License',
    'GNU GPL',
    'GNU LGPL',
    'GNU Lesser Public License',
    'GPL',
    'HPND',
    'IBM PUBLIC LICENSE VERSION 1.0',
    'ISC',
    'JPEG',
    'LGPL',
    'MIT',
    'MIT license',
    'MIT/X11 (BSD like)',
    'no notice',
    'noderivs',
    'none',
    'PD',
    'PD-debian',
    'PERLDOCS',
    'permissive-fsf',
    'permissive-nowarranty',
    'probably-PD',
    'Paul Vixie\'s license',
    'public-domain',
    'REGCOMP',
    'S2P',
    'SDBM-PUBLIC-DOMAIN',
    'TEXT-SOUNDEX',
    'TEXT-TABS',
    'The OpenLDAP Public License',
    'Unicode',
    'X11',
    'X11-2',
    'ZLIB',
]

LICENSES = [
    r'Permission to use, copy, modify, distribute, and sell this software and'
    r' its documentation for any purpose is hereby granted without fee,'
    r' provided that the above copyright notice appear in all copies and that'
    r' both that copyright notice and this permission notice appear in'
    r' supporting documentation, and that the name of the authors not be used'
    r' in advertising or publicity pertaining to distribution of the software'
    r' without specific, written prior permission. The authors makes no'
    r' representations about the suitability of this software for any purpose.'
    r' It is provided "as is" without express or implied warranty.',
    r'free software; you can redistribute it and/or modify it under the terms'
    r' of the GNU.*General Public License.*as published by the Free Software'
    r' Foundation',
    r'The main library is licensed under GNU Lesser General Public License'
    r' (LGPL) version 2.1+, Gnutls Extra (i.e. openssl wrapper library, and'
    r' library for code for "GnuTLS Inner Application" support) build system,'
    r' testsuite and commandline utilities are licenced under the GNU General'
    r' Public License version 3+. The Guile bindings use the same license as'
    r' the respective underlying library, i.e. LGPLv2.1+ for the main library'
    r' and GPLv3+ for Gnutls extra.',
    r'Permission is granted to anyone to use this.*for any purpose, including'
    r' commercial applications, and to alter it and redistribute it freely,'
    r' subject to the following restrictions',
    r'This software is released under the terms of the GNU.*General Public'
    r' License.*',
    r'All files in this package can be freely distributed and used according'
    r' to the terms of the GNU.*General Public License, either version 2 or (at'
    r' your opinion) any newer version. This is the same distribution policy as'
    r' for the Linux kernel itself -- see /usr/src/linux/COPYING for details.',
    r'You are free to distribute this.*under the terms of the BSD License',
    r'All files in this.*can be freely distributed and used according to the'
    r' terms of the GNU General Public License',
    r'all of the code is covered under the terms of the GPL.',
    r'is free software',
    r'You are free to distribute this software under the terms of the BSD'
    r' License.',
    r'is licensed under the BSD license',
    r'(is|are|be) free to distribute',
    r'may freely distribute',
    r'(is|are|be) freely distributed',
    r'.*is available under the terms of the GNU.*Public License',
    r'This data is licenced under 2 different licenses 1\) GNU General Public'
    r' License, version 2 or later 2\) XFree86 1.0 license This data can be'
    r' used freely under either license.',
    r'.*is in the public domain.',
    r'is covered under the terms of the GNU Public License.',
    r'redistribute it freely',
    r'the complete text of the GNU General Public License can be found in',
    r'free for commercial and non-commercial use as long as the following'
    r' conditions are aheared to',
    r'Permission to.*use.*distribute.*this.*for any purpose.*is.*granted',
    r'There are no restrictions on distributing unmodified copies of Vim except'
    r' that they must include this license text.',
    r'Redistribution and use.*(is|are) permitted',
    r'Permission is.*granted.*deal.*without restriction, including without'
    r' limitation the rights to use',
    r'All its programs.*may be redistributed under the terms of the GNU GPL,'
    r' Version 2 or later',
    r'is distributed under the GNU.*General Public License',
    r'This software is distributed under the GNU General Public License',
    r'This package is dual-licensed under the Academic Free License version'
    r' 2.1, and the GPL version 2.',
    r'may be used, modified and redistributed only under the terms of the GNU'
    r' General Public License',
    r'has been placed in the public domain',
    r'And licensed under the terms of the GPL license',
    r'are distributed under the terms of the GNU.*General Public License',
    r'The keys in the keyrings don\'t fall under any copyright. Everything else'
    r' in the package is covered by the GNU GPL.',
    r'the complete text of the GNU General Public License and of the GNU Lesser'
    r' Public License can be found in',
    r'THE ACCOMPANYING PROGRAM IS PROVIDED UNDER THE TERMS OF THIS IBM PUBLIC'
    r' LICENSE',
    r'THE ACCOMPANYING PROGRAM IS PROVIDED UNDER THE TERMS OF THIS COMMON'
    r' PUBLIC LICENSE',
    r'GNU LESSER GENERAL PUBLIC LICENSE',
    r'Permission is hereby granted.*to any person obtaining a copy of.*and'
    r' associated documentation files.*to deal in.*without restriction',
    r'Redistribution and use of this software and associated documentation'
    r' ("Software"), with or without modification, are permitted',
    r'This code is multi Licensed under all/any one of.*LGPLv2.*New Style'
    r' BSD.*MIT',
    r'LICENSE. You may copy and use the Software, subject to these conditions:'
    r' 1. This Software is licensed for use only in conjunction with Intel'
    r' component products. Use of the Software in conjunction with non-Intel'
    r' component products is not licensed hereunder.',
    r'Brocade Linux Fibre Channel HBA Firmware',
    r'QLogic Linux Fibre Channel HBA Firmware',
    r'Unlimited distribution and/or modification is allowed as long as this'
    r' copyright notice remains intact.',
    r'Permission is hereby granted to use.*this.*for any purpose',
    r'are in the public domain',
    r'is (available|distributed) under the terms of the GNU.*Public License',
    r'(libudev|libgudev|udev) is licensed under the GNU (L|)GPL',
    r'The Linux Console Tools are covered by the GPL',
    r'Some portions of os-prober',
    r'Netcat and the associated package is a product of Avian Research,'
    r' and is freely available in full source form with no restrictions'
    r' save an obligation to give credit where due.',
    r'Permission is hereby granted, without written agreement and'
    r' without licence or royalty fees, to use, copy, modify, and'
    r' distribute this software',
    r'Open Market permits you to use, copy, modify, distribute, and license'
    r' this Software and the Documentation for any purpose, provided that'
    r' existing copyright notices are retained in all copies and that this'
    r' notice is included verbatim in any distributions. No written agreement,'
    r' license, or royalty fee is required for any of the authorized uses.',
    r'This software is made available under the terms of *either* of the'
    r' licenses found in LICENSE.APACHE or LICENSE.BSD. Contributions to'
    r' cryptography are made under the terms of *both* these licenses.',
]


def IsValidLicenseName(license_check):
  for name in LICENSE_NAMES:
    m = re.search(r'(?:(?:License|Copyright)\s*:\s*{0})|'
                  '(?:(?:covered )*under (?:the )?{0})|'
                  '(?:under (?:the terms of )*the {0})'.format(name),
                  license_check, re.IGNORECASE)
    if m:
      return True
  return False


def IsValidLicenseText(license_check):
  for license_text in LICENSES:
    m = re.search(license_text, license_check, re.IGNORECASE)
    if m:
      return True
  return False


def IsValidLicense(license_check):
  if IsValidLicenseName(license_check) or IsValidLicenseText(license_check):
    return True


def main():
  # First pass
  problem_packages = []
  for filename in glob.glob(sys.argv[1]):
    try:
      license_check = open(filename).read()
      license_check = re.sub(r'(\*|#)*', r'', license_check)
      license_check = ' '.join(license_check.split())
      if not IsValidLicense(license_check):
        problem_packages.append(filename)
    except IOError:
      print('Error opening sys.argv[1].')
  if problem_packages:
    print(', '.join(map(str, problem_packages)))


main()
*/
