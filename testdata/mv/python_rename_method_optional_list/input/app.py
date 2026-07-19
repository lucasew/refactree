from typing import Optional

class A:
    def run(self) -> int:
        return 1

class B:
    def run(self) -> int:
        return 2

def use_opt_list(oa: Optional[list[A]], ob: Optional[list[B]]) -> int:
    return oa[0].run() + ob[0].run()

def use_opt_list_var(oa: Optional[list[A]], ob: Optional[list[B]]) -> int:
    ga = oa
    gb = ob
    return ga[0].run() + gb[0].run()

def use_opt_list_for(oa: Optional[list[A]], ob: Optional[list[B]]) -> int:
    n = 0
    for a in oa:
        n += a.run()
    for b in ob:
        n += b.run()
    return n

def use_union_list(oa: list[A] | None, ob: list[B] | None) -> int:
    return oa[0].run() + ob[0].run()

def use_opt_dict_list(oa: Optional[dict[str, list[A]]], ob: Optional[dict[str, list[B]]]) -> int:
    return oa["k"][0].run() + ob["k"][0].run()

def use_preserves_b(ob: Optional[list[B]]) -> int:
    return ob[0].run()
