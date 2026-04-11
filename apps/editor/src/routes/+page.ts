/**
 * +page.ts — root route load function.
 *
 * Redirects / → /projects.
 * Using `redirect` here (rather than onMount in the component) means the
 * redirect fires before the page renders, preventing any content flash.
 */

import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(307, "/projects");
};
