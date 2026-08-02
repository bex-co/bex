import { useContext } from "react";
import { LanguageContext } from "@/common/providers/language-provider";

export const useTranslations = () => useContext(LanguageContext);
