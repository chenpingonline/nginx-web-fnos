export const trafficRanges = [
  { value: 15, label: "15 分钟" },
  { value: 60, label: "1 小时" },
  { value: 300, label: "5 小时" },
  { value: 1440, label: "1 天" },
  { value: 10080, label: "7 天" },
  { value: 43200, label: "1 月" },
];

export function trafficPeriod(minutes: number) {
  if (minutes === 43200) return "最近 30 天";
  return `最近 ${trafficRanges.find((range) => range.value === minutes)?.label ?? `${minutes} 分钟`}`;
}
