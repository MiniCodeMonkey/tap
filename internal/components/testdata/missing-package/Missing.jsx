import { leftPad } from 'left-pad-not-installed';

export default function Missing() {
  return <div className="missing">{leftPad('hi', 4)}</div>;
}
