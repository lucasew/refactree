class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_list_index(items: list[A]):
    a = items[0]
    a.execute()


def use_list_index_b(items: list[B]):
    b = items[0]
    b.run()


def use_list_var_index(items: list[A], i: int):
    a = items[i]
    a.execute()


def use_dict_key(d: dict[str, A]):
    a = d["k"]
    a.execute()


def use_dict_key_b(d: dict[str, B]):
    b = d["k"]
    b.run()


def use_assigned_literal():
    xs = [A()]
    a = xs[0]
    a.execute()
    ys = [B()]
    b = ys[0]
    b.run()


def use_wrapper_then_index(items: list[A]):
    xs = list(items)
    a = xs[0]
    a.execute()


def use_wrapper_index_expr(items: list[A]):
    a = list(items)[0]
    a.execute()


def use_slice_not_element(items: list[A]):
    # slice yields a list — must not type xs as A (fail closed)
    xs = items[1:3]
    return xs
