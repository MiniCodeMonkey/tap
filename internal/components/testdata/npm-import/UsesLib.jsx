import { shout } from 'tiny-lib';

export default function UsesLib() {
  return <div className="uses-lib">{shout('hello')}</div>;
}
