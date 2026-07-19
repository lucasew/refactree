class A:
    def execute(self) -> int:
        return 1

class B:
    def run(self) -> int:
        return 2

def use_index(la: list[dict[str, A]], lb: list[dict[str, B]]) -> int:
    return la[0]["k"].execute() + lb[0]["k"].run()

def use_var(la: list[dict[str, A]], lb: list[dict[str, B]]) -> int:
    da = la[0]
    db = lb[0]
    return da["k"].execute() + db["k"].run()

def use_for(la: list[dict[str, A]], lb: list[dict[str, B]]) -> int:
    n = 0
    for da in la:
        n += da["k"].execute()
    for db in lb:
        n += db["k"].run()
    return n

def use_values(la: list[dict[str, A]], lb: list[dict[str, B]]) -> int:
    n = 0
    for a in la[0].values():
        n += a.execute()
    for b in lb[0].values():
        n += b.run()
    return n

def use_preserves_b(lb: list[dict[str, B]]) -> int:
    return lb[0]["k"].run()
