interface TsxProps {
  label: string;
}

export default function Tsx({ label }: TsxProps) {
  const count: number = label.length;
  return <div className="tsx">{label} ({count})</div>;
}
