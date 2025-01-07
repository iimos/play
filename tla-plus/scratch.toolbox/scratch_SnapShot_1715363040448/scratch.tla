------------------------------ MODULE scratch ------------------------------
EXTENDS Integers, TLC, Sequences, FiniteSets

ClockType == (0..23) \X (0..59) \X (0..59)

ToSeconds(t) == t[1]*3600 + t[2]*60 + t[3] 

ToClock(seconds) == CHOOSE x \in ClockType: ToSeconds(x) = seconds

\*Eval == ToSeconds(<<1,2,3>>)

Eval == ToClock(3723)

=============================================================================
\* Modification History
\* Last modified Fri May 10 20:43:54 MSK 2024 by iimoskalev
\* Created Fri May 10 12:08:09 MSK 2024 by iimoskalev
