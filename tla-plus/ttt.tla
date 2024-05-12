-------------------------------- MODULE ttt --------------------------------

EXTENDS Sequences, Integers, TLC

CONSTANTS Calls

(*--algorithm CallHistory

variables
    queue = <<>>
    store = <<>>

define
    AllCallsProcessed == <>[](
        \A i \in Calls: Len(SelectSeq(store, LAMBDA j: j = i)) = 1
    )
end define;

fair process Call \in Calls
begin CallSetup:
    skip
end process;

fair process Processor = "Processor"
begin Process:
    while true do 
        ProcessCDR:
            skip
    end while;
end process;

end algorithm *)
\* BEGIN TRANSLATION (chksum(pcal) = "2a8cd28e" /\ chksum(tla) = "48e3e053")
VARIABLES queue, pc

(* define statement *)
AllCallsProcessed == <>[](
    \A i \in Calls: Len(SelectSeq(store, LAMBDA j: j = i)) = 1
)


vars == << queue, pc >>

ProcSet == (Calls) \cup {"Processor"}

Init == (* Global variables *)
        /\ queue =         <<>>
        /\         store = <<>>
        /\ pc = [self \in ProcSet |-> CASE self \in Calls -> "CallSetup"
                                        [] self = "Processor" -> "Process"]

CallSetup(self) == /\ pc[self] = "CallSetup"
                   /\ TRUE
                   /\ pc' = [pc EXCEPT ![self] = "Done"]
                   /\ queue' = queue

Call(self) == CallSetup(self)

Process == /\ pc["Processor"] = "Process"
           /\ IF true
                 THEN /\ pc' = [pc EXCEPT !["Processor"] = "ProcessCDR"]
                 ELSE /\ pc' = [pc EXCEPT !["Processor"] = "Done"]
           /\ queue' = queue

ProcessCDR == /\ pc["Processor"] = "ProcessCDR"
              /\ TRUE
              /\ pc' = [pc EXCEPT !["Processor"] = "Process"]
              /\ queue' = queue

Processor == Process \/ ProcessCDR

(* Allow infinite stuttering to prevent deadlock on termination. *)
Terminating == /\ \A self \in ProcSet: pc[self] = "Done"
               /\ UNCHANGED vars

Next == Processor
           \/ (\E self \in Calls: Call(self))
           \/ Terminating

Spec == /\ Init /\ [][Next]_vars
        /\ \A self \in Calls : WF_vars(Call(self))
        /\ WF_vars(Processor)

Termination == <>(\A self \in ProcSet: pc[self] = "Done")

\* END TRANSLATION 

=============================================================================
\* Modification History
\* Last modified Fri May 10 00:02:10 MSK 2024 by iimoskalev
\* Created Thu May 09 22:56:20 MSK 2024 by iimoskalev
