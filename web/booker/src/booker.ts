const KEY = "yadori.bookerId";

export const currentBookerId = () => localStorage.getItem(KEY);

export const rememberBookerId = (id: string) => localStorage.setItem(KEY, id);
