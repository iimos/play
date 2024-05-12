---------------------- MODULE test ----------------------

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
    while true 
        ProcessCDR:
            skip
    end while;
end process;

end algorithm *)

=============================================================================