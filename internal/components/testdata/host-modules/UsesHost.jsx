import { useState } from 'react';
import { motion } from 'motion/react';
import { useStep } from 'tap';

export default function UsesHost() {
  const [count] = useState(0);
  const { step } = useStep();
  return (
    <motion.div className="uses-host">
      {count} {step}
    </motion.div>
  );
}
