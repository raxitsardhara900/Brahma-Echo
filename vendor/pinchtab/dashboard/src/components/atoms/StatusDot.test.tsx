import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import StatusDot from "./StatusDot";

describe("StatusDot", () => {
  it("renders online status with success color", () => {
    const { container } = render(<StatusDot status="online" />);
    const dot = container.querySelector(".rounded-full");
    expect(dot).toHaveClass("bg-success");
  });

  it("renders offline status with muted color", () => {
    const { container } = render(<StatusDot status="offline" />);
    const dot = container.querySelector(".rounded-full");
    expect(dot).toHaveClass("bg-text-muted");
  });

  it("renders warning status with warning color", () => {
    const { container } = render(<StatusDot status="warning" />);
    const dot = container.querySelector(".rounded-full");
    expect(dot).toHaveClass("bg-warning");
  });

  it("renders loading status with pulse animation", () => {
    const { container } = render(<StatusDot status="loading" />);
    const dot = container.querySelector(".rounded-full");
    expect(dot).toHaveClass("bg-primary", "animate-pulse");
  });

  it("renders label when provided", () => {
    render(<StatusDot status="online" label="Connected" />);
    expect(screen.getByText("Connected")).toBeInTheDocument();
  });

  it("does not render label when not provided", () => {
    const { container } = render(<StatusDot status="online" />);
    const label = container.querySelector(".text-xs");
    expect(label).not.toBeInTheDocument();
  });

  it("applies small size", () => {
    const { container } = render(<StatusDot status="online" size="sm" />);
    const dot = container.querySelector(".rounded-full");
    expect(dot).toHaveClass("h-2", "w-2");
  });

  it("applies medium size by default", () => {
    const { container } = render(<StatusDot status="online" />);
    const dot = container.querySelector(".rounded-full");
    expect(dot).toHaveClass("h-2.5", "w-2.5");
  });
});
