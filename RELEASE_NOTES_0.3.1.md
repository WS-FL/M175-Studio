# M175 Studio 0.3.1 beta

Fix device-name overflow in the lower-left scanner selector. Long Bonjour names now stay inside the sidebar, use an ellipsis, and leave the dropdown arrow visible. Hover the device area for the full name and endpoint. English, Chinese and unusually long names are tested at normal and minimum window widths.

The actual Bonjour name observed during diagnosis was valid text; the reported visual corruption came from an inline title without effective truncation and inherited horizontal margins on a full-width button. Discovery and the scanner protocol remain unchanged.

Includes the 0.3.0 discovery and inline rename features. Universal Apple Silicon / Intel package, ad-hoc signed and not notarized. Automated Go scan regressions and browser feature/layout tests passed. No physical scan was performed for this display fix. No GitHub Release is published.
