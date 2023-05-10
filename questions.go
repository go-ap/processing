package processing

import (
	vocab "github.com/go-ap/activitypub"
)

// QuestionActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-questions
//
// The Questions use case primarily deals with representing inquiries of any type.
// See 5.4 Representing Questions for more information: https://www.w3.org/TR/activitystreams-vocabulary/#questions
func QuestionActivity(l WriteStore, q *vocab.Question) (*vocab.Question, error) {
	// NOTE(marius): this behaviour is not in accordance to the spec.
	// Saving the question items as individual objects makes sense to me,
	// but it's not mention in either of the documents we're basing this implementation on.
	// I can't think of any reason why saving them might be a _bad_ idea, so for the moment I'll leave it like this.
	if q.AnyOf != nil {
		return q, vocab.OnItemCollection(q.AnyOf, saveQuestionAnswers(l, q))
	}
	if q.OneOf != nil {
		return q, vocab.OnItemCollection(q.OneOf, saveQuestionAnswers(l, q))
	}
	return q, nil
}

func saveQuestionAnswers(l WriteStore, q *vocab.Question) func(col *vocab.ItemCollection) error {
	return func(col *vocab.ItemCollection) error {
		var err error
		for _, ans := range col.Collection() {
			if iri := ans.GetLink(); len(iri) == 0 {
				err = SetIDIfMissing(ans, nil, q)
			}

			vocab.OnActivity(q, func(act *vocab.Activity) error {
				return updateCreateActivityObject(l, ans, act)
			})

			ans, err = l.Save(ans)
		}
		return err
	}
}
