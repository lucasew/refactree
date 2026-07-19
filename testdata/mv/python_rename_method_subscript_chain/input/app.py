class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_list_index_chain(as_: list[A]):
    as_[0].run()


def use_list_index_chain_b(bs: list[B]):
    bs[0].run()


def use_list_var_index_chain(as_: list[A], i: int):
    as_[i].run()


def use_dict_key_chain(am: dict[str, A]):
    am["k"].run()


def use_dict_key_chain_b(bm: dict[str, B]):
    bm["k"].run()


def use_assigned_literal_chain():
    xs = [A()]
    xs[0].run()
    ys = [B()]
    ys[0].run()


def use_wrapper_index_chain(as_: list[A]):
    list(as_)[0].run()


def use_wrapper_index_chain_b(bs: list[B]):
    list(bs)[0].run()


def use_paren_index_chain(as_: list[A]):
    (as_)[0].run()


def use_assign_still_ok(as_: list[A], bm: dict[str, B]):
    a = as_[0]
    a.run()
    b = bm["k"]
    b.run()
