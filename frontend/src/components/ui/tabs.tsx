import { useEffect, useRef, useState, type ComponentProps } from "react";
import { Tabs as TabsPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";

export function Tabs(props: ComponentProps<typeof TabsPrimitive.Root>) {
  return <TabsPrimitive.Root data-slot="tabs" {...props} />;
}
export function TabsList({
  className,
  ref,
  ...props
}: ComponentProps<typeof TabsPrimitive.List>) {
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    function revealSelected() {
      const selected = list!.querySelector<HTMLElement>(
        '[data-state="active"]',
      );
      if (!selected) return;
      const bounds = list!.getBoundingClientRect();
      const item = selected.getBoundingClientRect();
      if (item.left < bounds.left) list!.scrollLeft -= bounds.left - item.left;
      else if (item.right > bounds.right)
        list!.scrollLeft += item.right - bounds.right;
    }
    const resize = new ResizeObserver(revealSelected);
    const selection = new MutationObserver(revealSelected);
    resize.observe(list);
    selection.observe(list, {
      subtree: true,
      attributes: true,
      attributeFilter: ["data-state"],
    });
    revealSelected();
    return () => {
      resize.disconnect();
      selection.disconnect();
    };
  }, []);
  return (
    <TabsPrimitive.List
      ref={(node) => {
        listRef.current = node;
        if (typeof ref === "function") return ref(node);
        if (ref) ref.current = node;
      }}
      data-slot="tabs-list"
      className={cn("ui-tabs-list", className)}
      {...props}
    />
  );
}
export function TabsTrigger({
  className,
  onPointerDown,
  onKeyDown,
  onBlur,
  ...props
}: ComponentProps<typeof TabsPrimitive.Trigger>) {
  const [pointerFocus, setPointerFocus] = useState(false);
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      data-pointer-focus={pointerFocus || undefined}
      className={cn("ui-tabs-trigger", className)}
      onPointerDown={(event) => {
        setPointerFocus(true);
        onPointerDown?.(event);
      }}
      onKeyDown={(event) => {
        setPointerFocus(false);
        onKeyDown?.(event);
      }}
      onBlur={(event) => {
        setPointerFocus(false);
        onBlur?.(event);
      }}
      {...props}
    />
  );
}
export function TabsContent({
  className,
  ...props
}: ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      data-slot="tabs-content"
      className={cn("ui-tabs-content", className)}
      {...props}
    />
  );
}
