import { assertEquals } from "@std/assert";
import { type ChangeSource, reviewChangedBannerText } from "./notification.ts";

function sources(...given: ChangeSource[]): Set<ChangeSource> {
  return new Set(given);
}

Deno.test("reviewChangedBannerText reports new commits alone", () => {
  const actual = reviewChangedBannerText(sources("commits"));
  assertEquals(actual, "New code was committed to this branch.");
});

Deno.test("reviewChangedBannerText reports external state edits alone", () => {
  const actual = reviewChangedBannerText(sources("state"));
  assertEquals(actual, "Changes to the review have been made.");
});

Deno.test("reviewChangedBannerText reports both when both are pending", () => {
  const actual = reviewChangedBannerText(sources("commits", "state"));
  assertEquals(
    actual,
    "New code was committed and changes to the review have been made.",
  );
});

Deno.test("reviewChangedBannerText returns null when nothing is pending", () => {
  assertEquals(reviewChangedBannerText(sources()), null);
});
