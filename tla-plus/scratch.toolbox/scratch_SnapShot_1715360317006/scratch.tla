------------------------------ MODULE scratch ------------------------------
EXTENDS Integers, TLC, Sequences, FiniteSets

Eval == CHOOSE x \in 1..20: x % 2 = 0

=============================================================================
\* Modification History
\* Last modified Fri May 10 19:58:28 MSK 2024 by iimoskalev
\* Created Fri May 10 12:08:09 MSK 2024 by iimoskalev
