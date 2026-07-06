import { Label } from "@/common/components/ui/label.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { TabsList, TabsTrigger } from "@/common/components/ui/tabs.tsx";

type TabOptions = {
  label: string;
  value: string;
};

export type ResponsiveTabTriggerListProps = {
  selectedTab: string;
  setSelectedTab: (tab: string) => void;
  tabOptions: TabOptions[];
  className?: string;
};

export const ResponsiveTabTriggerList = ({
  selectedTab,
  setSelectedTab,
  tabOptions,
  className,
}: ResponsiveTabTriggerListProps) => {
  return (
    <div className={className}>
      <Label htmlFor="view-selector" className="sr-only">
        View
      </Label>
      <Select value={selectedTab} onValueChange={setSelectedTab}>
        <SelectTrigger
          className="flex w-fit lg:hidden"
          size="sm"
          id="view-selector"
        >
          <SelectValue placeholder="Select a view" />
        </SelectTrigger>
        <SelectContent>
          {tabOptions.map((tab) => (
            <SelectItem key={tab.value} value={tab.value}>
              {tab.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <TabsList className="**:data-[slot=badge]:bg-muted-foreground/30 hidden **:data-[slot=badge]:size-5 **:data-[slot=badge]:rounded-full **:data-[slot=badge]:px-1 lg:flex">
        {tabOptions.map((tab) => (
          <TabsTrigger key={tab.value} value={tab.value}>
            {tab.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </div>
  );
};
