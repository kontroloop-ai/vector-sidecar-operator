#!/usr/bin/env python3
"""
Fix CRD validation issues with resource claims x-kubernetes-map-type.
This script adds x-kubernetes-map-type: atomic to items in claims sections.

The structure we're looking for:
  claims:
    items:
      properties:
        ...
      required:
      - name
      type: object    <-- Need to add x-kubernetes-map-type: atomic here
    type: array
    x-kubernetes-list-type: set
"""

def fix_claims(filename):
    with open(filename, 'r') as f:
        lines = f.readlines()

    fixed_lines = []
    i = 0
    in_claims_items = False
    claims_indent = 0

    while i < len(lines):
        line = lines[i]

        # Detect when we enter a claims section
        if "claims:" in line and not in_claims_items:
            in_claims_items = True
            claims_indent = len(line) - len(line.lstrip())
            fixed_lines.append(line)
            i += 1
            continue

        # If we're in a claims section
        if in_claims_items:
            # Check if we've exited the claims section (back to same or lower indent)
            current_indent = len(line) - len(line.lstrip())
            if line.strip() and current_indent <= claims_indent:
                in_claims_items = False

            # Look for "type: object" inside items block, followed by "type: array" at outer level
            if line.strip() == "type: object":
                fixed_lines.append(line)

                # Check if next significant lines include "type: array" (not nested deeper)
                # and "x-kubernetes-list-type: set"
                j = i + 1
                found_array = False
                found_list_type = False

                while j < len(lines) and j < i + 5:
                    next_line = lines[j]
                    if "type: array" in next_line:
                        found_array = True
                    if "x-kubernetes-list-type: set" in next_line:
                        found_list_type = True
                        break
                    j += 1

                # If we found both, this is a claims items block that needs fixing
                if found_array and found_list_type:
                    # Check if x-kubernetes-map-type already exists
                    has_map_type = False
                    for k in range(max(0, i-5), min(len(lines), i+5)):
                        if "x-kubernetes-map-type" in lines[k] and k != i:
                            # Make sure it's at the right level (same as type: object)
                            if abs((len(lines[k]) - len(lines[k].lstrip())) - (len(line) - len(line.lstrip()))) < 2:
                                has_map_type = True
                                break

                    if not has_map_type:
                        # Add x-kubernetes-map-type with same indentation as "type: object"
                        indent = len(line) - len(line.lstrip())
                        map_type_line = " " * indent + "x-kubernetes-map-type: atomic\n"
                        fixed_lines.append(map_type_line)
                        i += 1
                        continue

        fixed_lines.append(line)
        i += 1

    with open(filename, 'w') as f:
        f.writelines(fixed_lines)

    print(f"Fixed CRD: {filename}")

if __name__ == "__main__":
    fix_claims("config/crd/bases/observability.kontroloop.ai_vectorsidecars.yaml")
