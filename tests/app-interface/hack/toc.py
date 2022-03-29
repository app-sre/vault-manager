#!/usr/bin/env python

# This program is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 2 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
#
# See LICENSE for more details.
#
# Copyright: Red Hat Inc. 2020
# Author: Amador Pahim <apahim@redhat.com>

"""Table of Contents Generator"""

import re


with open('README.md') as file_obj:
    links = []

    # Code sections are enclosed in ```
    # The comments there may mess the ToC, so we
    # have to ignore those sections
    code_section = False

    for line in file_obj:

        # Mark the beginning of a code section
        if not code_section and line.startswith('```'):
            code_section = True
            continue

        # Mark the end of a code seciont
        if code_section:
            if line.startswith('```'):
                code_section = False
            continue

        # Only interested in lines starting with #
        if not line.startswith('#'):
            continue

        elements = line.split()
        hashes = elements[0]
        title_el = elements[1:]

        # Converting hashes into whitespaces:
        # - One hash -> no whitespace
        # - Each additional hash -> 2 additional whitespaces
        # - The "- " goes after the whitespaces
        # Example:
        # "#"    -> "- "
        # "##"   -> "  - "
        # "###"  -> "    - "
        # "####" -> "      - "
        level = (' ' * (2 * (len(hashes) - 1))) + '- '

        # Only alphanumerics and "-" are used for links
        link_el =  [re.sub('[^a-zA-Z0-9-]', '', word) for word in title_el]

        " ".join(elements[1:])
        link = "-".join(link_el).lower()

        # Repeated links will be numbered
        new_link = link
        counter = 1
        while True:
            # First occurrence of a link is not numbered
            if new_link not in links:
                links.append(new_link)
                break

            # Starting from the second occurrence, the counter is used
            new_link = '{}-{}'.format(link, counter)
            counter += 1

        # Putting all together
        title = " ".join(title_el)
        print('{}[{}](#{})'.format(level, title, new_link))
