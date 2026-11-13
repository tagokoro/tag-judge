"use client";

import { useId } from "react";
import type { Category } from "@/lib/types";
import { Field } from "./ui";

/** The user's category with this name, compared the way the server does (trimmed, ignoring case). */
function findCategory(categories: Category[], name: string): Category | undefined {
  const wanted = name.trim().toLowerCase();
  return categories.find((category) => category.name.toLowerCase() === wanted);
}

/**
 * The category a library is filed under: pick one of the user's categories or
 * type a new name. The server uses the category by that name or creates it, so
 * nobody manages categories by hand.
 */
export function CategoryField({
  value,
  onChange,
  categories,
}: {
  value: string;
  onChange: (value: string) => void;
  categories: Category[];
}) {
  const listId = useId();
  const isNew = value.trim() !== "" && findCategory(categories, value) === undefined;
  return (
    <Field
      label="カテゴリ"
      hint={
        isNew
          ? `「${value.trim()}」は新しいカテゴリとして作られます。`
          : "既存のカテゴリから選ぶか、新しい名前を入力します（1〜64 文字）。"
      }
    >
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        list={listId}
        placeholder="グラフ"
        autoComplete="off"
      />
      <datalist id={listId}>
        {categories.map((category) => (
          <option key={category.id} value={category.name} />
        ))}
      </datalist>
    </Field>
  );
}
