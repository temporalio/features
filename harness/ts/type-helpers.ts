/** An object T with any nested values of type ToReplace replaced with ReplaceWith */
export type ReplaceNested<T, ToReplace, ReplaceWith> = T extends (...args: any[]) => any
  ? T
  : [keyof T] extends [never]
    ? T
    : T extends Record<string, string> // Special exception for Nexus Headers.
      ? T
      : T extends { [k: string]: ToReplace }
        ? {
            [P in keyof T]: ReplaceNested<T[P], ToReplace, ReplaceWith>;
          }
        : T extends ToReplace
          ? ReplaceWith | Exclude<T, ToReplace>
          : {
              [P in keyof T]: ReplaceNested<T[P], ToReplace, ReplaceWith>;
            };
