import { describe, expect, it } from "vitest";
import type { Profile } from "../generated/types";
import { formatProfileBytes, groupBytes, groupProfiles } from "./profileGroups";

describe("profile grouping helpers", () => {
  it("groups on the flags rather than the name", () => {
    const groups = groupProfiles([
      { name: "default", running: false } as Profile,
      { name: "looks.quarantine-1700000000", running: false } as Profile,
      { name: "real", running: false, quarantined: true } as Profile,
      { name: "inst", running: false, temporary: true } as Profile,
    ]);

    expect(groups.user.map((p) => p.name)).toEqual([
      "default",
      "looks.quarantine-1700000000",
    ]);
    expect(groups.quarantined.map((p) => p.name)).toEqual(["real"]);
    expect(groups.temporary.map((p) => p.name)).toEqual(["inst"]);
  });

  it("formats bytes the way the CLI does", () => {
    expect(formatProfileBytes(512)).toBe("512 B");
    expect(formatProfileBytes(1024)).toBe("1.0 KB");
    expect(formatProfileBytes(4 * 1024 * 1024)).toBe("4.0 MB");
    expect(formatProfileBytes(2 * 1024 * 1024 * 1024)).toBe("2.0 GB");
  });
});

describe("groupBytes", () => {
  it("sums diskUsage, the field the CLI totals", () => {
    expect(
      groupBytes([
        { name: "a", running: false, diskUsage: 1024, sizeMB: 99 } as Profile,
        { name: "b", running: false, diskUsage: 2048, sizeMB: 99 } as Profile,
        { name: "c", running: false } as Profile,
      ]),
    ).toBe(3072);
  });
});
